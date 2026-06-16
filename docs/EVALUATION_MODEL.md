# Position Evaluation Model

Status: current implementation.

This document describes how the Durak position evaluator works today. It is a
seat-view heuristic engine: it scores what one seat can know, ranks legal
actions, and explains why one action is better than another. It is not a win
probability model and it does not inspect hidden opponent hands.

## Purpose

The evaluator gives the game a chess-engine-like signal for positions and
moves. It supports three current use cases:

- `heuristic` bot decisions in offline play;
- `arena -eval` aggregate move-quality statistics;
- `analyze` post-game review for matches stored in SQLite.

The score answers this question: "How good is this position for the evaluated
seat if both players still need to choose good actions?" It does not answer
"What is the exact chance to win?"

## Public API Shape

The evaluator lives in `internal/app/evaluation`. It depends on `internal/app`
and `internal/domain`; those parent packages do not depend on the evaluator.
This keeps the domain state machine independent from strategy code.

The main entry points are:

- `BuildHiddenCards(decision, discard)` builds the seat-view hidden-card model.
- `Evaluate(decision, hidden)` returns a positional score and feature list.
- `RankActions(decision, hidden)` ranks all legal actions for the active seat.
- `AnalyzeMatchFromHistory(ctx, reader, matchID)` replays a stored match and
  scores every saved action.
- `app.PublicCardHistory` maintains runtime public memory for active sessions
  and replay analysis.

Adapters consume these APIs. The CLI formats results, but it does not calculate
scores itself.

## Information Model

The evaluator receives an `app.DecisionContext`. That context contains:

- evaluated seat id;
- phase, attacker, defender, trump suit, and trump indicator;
- public table cards;
- visible hand sizes;
- stock count and discard count;
- the evaluated seat's private hand;
- legal actions for that seat.

The evaluator may also receive explicit discard cards. Live arena scoring does
not read hidden hands or stock order. It receives runtime public memory from the
application layer. Stored match analysis reconstructs the same fair public
memory from replayed events before each scored action.

The hidden-card model splits the 36-card deck into:

- known cards: own hand, table cards, visible trump indicator, discard cards,
  and cards an opponent visibly took from the table;
- unknown pool: every card not known to this seat;
- known-held cards: cards known to sit in a specific seat's hand.

For a specific unknown card, the rough opponent-hand probability is:

```text
opponent hand size / unknown pool size
```

For a card known to sit in an opponent hand, the probability is `1`. For a card
known in the evaluated seat's own hand, table, or discard, the probability is
`0`.

This model becomes sharper in late game. When stock is empty and many cards are
discarded, the unknown pool shrinks and the evaluator can reason with higher
confidence.

## Score Scale

Scores use a bounded integer scale:

- `0`: roughly equal;
- `+100`: small advantage;
- `+300`: clear advantage;
- `+700`: nearly decisive advantage;
- `+1000`: practical upper bound;
- `-1000`: practical lower bound.

Positive values favor the evaluated seat. Negative values favor the opponents.
The evaluator clamps every public score into `[-1000, +1000]`.

The evaluator also returns confidence from `0..100`. Confidence is lower when
the unknown pool is large. Confidence does not change the score directly except
through the explicit uncertainty penalty.

## Static Position Features

The static evaluator sums feature contributions and clamps the result.

`material_pressure` compares the evaluated hand size with visible opponent hand
sizes. Fewer cards are better. The current v1 formula uses average visible
opponent hand size minus own hand size, multiplied by a fixed weight.

`trump_strength` rewards trump cards in hand. High trumps add more than low
trumps. This is deliberately simple: it values ownership of trump power without
trying to predict every future exchange.

`defense_coverage` applies when the evaluated seat is defending. Each pending
attack card is checked with `domain.CanBeat`. Covered attacks add score;
uncovered attacks subtract more score because taking is expensive.

`attack_pressure` counts legal attack, throw-in, and transfer actions. More
pressure options are positive because they let the seat shed cards or force a
defender response.

`role_phase_risk` gives a small bonus to the attacker, a small penalty to the
defender, and a larger penalty when the defender is already in the taking phase.

`endgame_pressure` increases the value of material lead when the stock is low or
empty. In late game, each card left in hand matters more.

`uncertainty_penalty` subtracts score when confidence is low. It prevents early
positions with many unknown cards from looking too certain.

## Action Scoring

`RankActions` scores each legal action as:

```text
current static score + action-specific delta
```

It does not clone `Session`, simulate draws, or expose hidden stock order. This
is intentional. V1 action scoring stays seat-view-safe and does not model hidden
refill outcomes.

Action deltas are explicit:

- defending is strongly positive;
- the cheapest sufficient defense is preferred;
- spending a high trump when a non-trump defense works is penalized;
- taking is strongly negative, especially when legal defense exists;
- low non-trump attacks are preferred unless memory or endgame pressure changes
  the ranking;
- throw-ins receive a separate pressure score, especially when the defender is
  taking or when stock is low;
- high cards and trumps are expensive to throw;
- finishing a successful defense is rewarded only when available throw-in
  pressure is not more valuable;
- passing a throw-in is mildly negative when useful pressure exists.

After scoring, actions are sorted best first. Each action receives:

- `score`: resulting action score;
- `delta`: score change versus the current static position;
- `loss`: score lost compared with the best ranked action;
- `quality`: a label derived from loss.

Current quality thresholds:

- `best`: loss `0..20`;
- `good`: loss `21..80`;
- `inaccuracy`: loss `81..180`;
- `mistake`: loss `181..350`;
- `blunder`: loss above `350`;
- `brilliant`: reserved for a later search-based evaluator.

## Bot Usage

The `heuristic` bot builds a hidden-card model from its `TurnContext`, ranks the
legal actions, and returns the top action. The application still validates the
chosen action through the same path as human and AI actions.

Example:

```sh
go run ./cmd/durak arena -matches 100 -seed 42 -p0 heuristic -p1 simple
```

## Live Calibration

Arena mode can score accepted actions during headless play:

```sh
go run ./cmd/durak arena -matches 100 -seed 42 -eval -p0 heuristic -p1 simple
```

The output includes aggregate move-quality counters:

```text
Evaluation: turns=291 avg_loss=12 best=255 good=15 inaccuracy=21 mistake=0 blunder=0 illegal=0 seats=[0:turns=137 avg_loss=0 blunder=0,1:turns=154 avg_loss=23 blunder=0]
```

This is useful for comparing controllers. A heuristic seat should usually have
lower average loss than `simple` or `random` because it chooses actions from the
same ranking model.

## Stored Match Analysis

SQLite match history can be reviewed after a run:

```sh
go run ./cmd/durak analyze -db .cache/durak.db -match-id demo-1 -limit 5
```

The analyzer reads internal events, reconstructs the match step by step, builds
the acting seat's decision context before each stored action, and ranks that
stored action against legal alternatives. It prints the highest-loss moves:

```text
Analysis: match=demo-1 moves=76 avg_loss=11 best=65 good=5 inaccuracy=6 mistake=0 blunder=0 concessions=0
Worst moves:
  turn=53 seq=80 seat=1 loss=174 quality=inaccuracy rank=3/3 action=throw KD best=done score=322 confidence=90
```

This gives us a practical review loop before a training UI exists.

## Current Limits

The evaluator is static plus action deltas. It does not search reply trees. It
does not compute win probability. It does not use oracle hands. It does not
persist score tables.

Live arena scoring keeps public memory in the active session. It does not write
memory snapshots to SQLite. Stored analysis rebuilds the same fair memory from
replayed public events instead of reading hidden hands.

The weights are hand-tuned constants. They are meant to be inspected, tested,
and calibrated with arena and stored-match reports.

## Calibration Workflow

Use arena to generate data:

```sh
go run ./cmd/durak arena -matches 100 -seed 42 -eval -db .cache/eval.db -match-id eval-2p -p0 heuristic -p1 simple
```

Review individual games:

```sh
go run ./cmd/durak analyze -db .cache/eval.db -match-id eval-2p -limit 10
```

Look for repeated patterns, not one-off decisions. Good calibration targets are:

- throw-in versus `done`;
- take versus defend;
- spending trump versus preserving trump;
- endgame card-count pressure;
- transfer value;
- uncertainty penalties in early game.

After changing a weight or feature, add a focused unit test for the pattern and
rerun arena comparisons.

## First Calibration Snapshot

The first offline calibration run used 20 matches for each table size from 2 to
6 seats. It mixed `heuristic`, `simple`, and `random` controllers and stored
SQLite histories under `.cache/evaluation-runs/`.

| Run | Controllers | Results | Evaluation signal |
| --- | --- | --- | --- |
| `offline-2p` | `heuristic`, `simple` | seat0=3, seat1=17 | heuristic avg_loss=0, simple avg_loss=19 |
| `offline-3p` | `heuristic`, `simple`, `random` | seat0=17, seat1=3, seat2=0 | random avg_loss=84, 47 blunders |
| `offline-4p` | `heuristic`, `simple`, `random`, `heuristic` | seat0=19, seat1=1 | random avg_loss=70, 34 blunders |
| `offline-5p` | `heuristic`, `simple`, `random`, `heuristic`, `simple` | seat0=16, seat1=2, draws=2 | random avg_loss=65, 24 blunders |
| `offline-6p` | `heuristic`, `simple`, `random`, `heuristic`, `simple`, `random` | seat0=19, seat1=1 | random seats avg_loss about 70, 79 total blunders |

The repeated worst moves are useful. The analyzer often flags:

- throwing into a round when `done` would end a successful defense;
- taking when a defense exists;
- defending with an expensive card when a cheaper card works;
- passing when a useful throw-in is available.

The 2-player result is the main warning sign. The `heuristic` bot has zero loss
against its own ranking, but it lost heavily to `simple` in that run. This means
the current one-ply ranking is internally consistent but not yet calibrated for
longer two-player outcomes.

AI smoke used `ai-openai` from `.env`. One 2-player match completed and produced
70 analyzed moves with `avg_loss=9`. A 3-player AI run hit provider rate limits
after a partial stream, so that data is useful only for inspecting early move
patterns, not for win-rate conclusions.

## v2 Pressure and Memory Snapshot

The first v2 implementation added runtime public card memory and pressure-aware
throw-in scoring. It removed the original failure where `heuristic` almost never
threw in. On the mirrored 1000+1000 match seed 202 benchmark, the result is now
near parity:

| Run | Results | Signal |
| --- | --- | --- |
| `heuristic` vs `simple` | heuristic=464, simple=471, draws=65 | heuristic no longer collapses, but does not lead |
| `simple` vs `heuristic` | simple=485, heuristic=465, draws=50 | mirror stays close |

This means v2 fixed pressure awareness but did not satisfy the desired 55-60%
win-rate target against `simple`. The next evaluator step should use public
memory more directly, likely through a narrow one-round or endgame search, rather
than broad weight changes.

## Next Directions

The next technical step is calibration, not TUI. We should run mixed controller
sets, review recurring worst moves, and adjust feature weights. After that, the
natural engine steps are:

- use known-card memory directly in one-round or endgame action scoring;
- add aggregate score reports without persisting runtime memory snapshots;
- implement probabilistic shallow search over plausible hidden hands;
- expose training and game-review screens once the score is trustworthy.
