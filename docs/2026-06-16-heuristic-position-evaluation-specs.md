# Heuristic Position Evaluation Spec

Status: Initial v1 implemented. Static seat-view scoring, hidden-card modeling,
action ranking, and the first `heuristic` bot are in place; probabilistic
shallow search and oracle analysis remain future work.

## Purpose

The next strategic layer should give Durak a chess-engine-like position
evaluation without pretending that the game has perfect information. The engine
will score a position from one seat's point of view, rank legal actions, and
later drive stronger offline bots and post-game move analysis.

This is not a win-probability model. A player can still blunder, concede, or
miss a winning line. The score is a practical positional signal that helps
compare states and moves.

## Core Decisions

- The first evaluator is a seat-view evaluator. It receives only information
  available to the evaluated seat: own hand, public table state, trump, discard,
  stock count, visible hand sizes when the mode exposes them, and public action
  history.
- The first implementation does not use oracle data such as hidden opponent
  hands or stock order. Oracle analysis can be added later for post-game
  training, tests, or engine calibration.
- The main score is an integer positional score on a chess-like scale:
  - `0` means roughly equal;
  - `+100` means a small advantage for the evaluated seat;
  - `+300` means a clear advantage;
  - `+700` means a nearly decisive advantage;
  - `+1000` and `-1000` represent practically decisive positions without
    becoming win probabilities.
- Action scoring starts with static evaluation plus one ply: evaluate the
  current seat-view position, apply each legal action, and evaluate the resulting
  seat-view position.
- Full shallow search is a later layer. Because Durak has hidden information,
  it should use a probabilistic hidden-card model instead of chess-style
  perfect-information minimax.

## Information Model

The evaluator should build a hidden-card model before it scores a position.
For the evaluated seat, cards fall into three groups:

- known cards: own hand, public table cards, discard, and public revealed cards;
- impossible cards: cards that cannot be in an opponent hand because they are
  known elsewhere;
- unknown pool: cards that may be in an opponent hand or the remaining stock.

In a two-player 36-card match at the start, the evaluated player knows six own
cards and the visible trump indicator. The opponent's six cards are drawn from
the remaining unknown pool, so the chance of a specific unknown card being in
the opponent's hand is approximately `opponent_hand_size / unknown_pool_size`,
not `1 / 36`.

The model should become sharper as public information accumulates. Played cards
leave the unknown pool. Cards in discard are impossible. Empty stock plus known
own hand, table, and discard can make an opponent hand fully reconstructable in
late-game two-player positions.

## Evaluation Features

The first feature set should stay explicit and explainable. Candidate features:

- material pressure: own hand size, visible opponent hand sizes, and stock count;
- trump strength: number and rank distribution of own trumps;
- defense coverage: how well own cards can beat plausible attacks;
- attack pressure: ranks that can be attacked or thrown in now;
- table role: attacker, defender, throw-in participant, taking player, or idle
  seat;
- phase risk: cost of being forced to take, value of finishing defense, and
  cost of spending high trumps too early;
- endgame shape: stock empty or nearly empty, known-card certainty, and whether
  low cards can be shed safely;
- uncertainty penalty: score confidence should drop when important opponent
  cards are still unknown.

Feature weights should start as constants, not learned parameters. The first
goal is a stable, testable heuristic that can explain its score.

## Action Scoring

Action scoring should evaluate every legal action for the active seat and return
a ranked list. Each action result should include:

- the action;
- resulting positional score;
- delta from the current position;
- loss from the best action;
- optional human-readable feature contributions.

Move quality labels can be derived from loss versus the best action:

- `best`: loss `0..20`;
- `good`: loss `21..80`;
- `inaccuracy`: loss `81..180`;
- `mistake`: loss `181..350`;
- `blunder`: loss above `350`;
- `brilliant`: reserved for later, when search can detect strong non-obvious
  moves.

The exact thresholds are initial constants. Arena and stored match history can
later calibrate them.

## Package Boundaries

The evaluator belongs above the domain core. Domain code should continue to own
legal actions and state transitions, but it should not import evaluator code or
strategy packages.

The first implementation can live in `internal/app` as adapter-neutral
application logic over `SeatView`, `DecisionContext`, public history, and legal
actions. Bot adapters can consume evaluation results through an app-level
interface. CLI and future training views can render evaluation output, but they
must not calculate scores themselves.

## Out of Scope

- Win probability.
- Learned weights or model training.
- Oracle analysis using hidden hands or stock order.
- Full shallow search or expectimax.
- Persistent score tables in SQLite.
- TUI, SSH, Wish, daemon integration.
- User-facing training UI.

## Future Direction

The next layer after static plus one-ply scoring is probabilistic shallow search.
For each candidate action, the engine should reason over plausible opponent
hands and likely replies, then aggregate the outcomes. This is closer to
expectimax over hidden-card distributions than chess minimax over a fully
visible board.

SQLite history can later support calibration: compare heuristic scores with
arena outcomes, identify recurring bad moves, and tune weights without changing
the domain rules.

## Testing Requirements

- Unit tests for hidden-card pool construction.
- Unit tests for score sign and scale on clear positions.
- Unit tests for action ranking on simple forced choices.
- Regression tests for late-game positions where hidden information collapses
  to known opponent cards.
- Arena smoke tests comparing `simple`, `random`, and the first heuristic bot
  after the bot uses the evaluator.
