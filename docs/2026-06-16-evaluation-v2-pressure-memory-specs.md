# Evaluation v2 Pressure and Memory Spec

Status: initial implementation added. Public runtime memory and pressure-aware
throw-in scoring are implemented; the 55-60% win-rate target against `simple`
is not yet reached.

## Purpose

Evaluation v2 should fix the main weakness found in the first calibration pass:
the evaluator underrates throw-in pressure and overrates ending a defended
round. In two-player games this makes the `heuristic` bot lose to `simple`,
even though `simple` has higher action loss under the current model.

The goal is not to build a perfect Durak engine. The goal is a stronger,
seat-view evaluator that reflects the real tactical shape of the game: force
the opponent to take cards, preserve trump strength, track known cards, and
adapt its judgment as the stock empties.

## Problem Statement

The first 2-player arena benchmarks exposed a mismatch between model loss and
match outcome.

```text
heuristic vs simple, 500 matches: heuristic 73, simple 374, draws 53
simple vs heuristic, 500 matches: simple 369, heuristic 73, draws 58
```

The action profile explains the failure. In the saved 20-match 2-player run,
`simple` made 146 throw-ins while `heuristic` made only 4. The current analyzer
often labels those throw-ins as inaccuracies because it prefers `done`, but
`simple` wins by keeping the defender under pressure.

Evaluation v2 should treat throw-ins as a central action class, not as a minor
variant of attacking with a low card.

## Core Decisions

- Keep v2 as a heuristic evaluator, not a win-probability model.
- Keep the first implementation seat-view fair: live bots must not inspect
  hidden opponent hands or future stock order.
- Consume app-level public card memory before adding full probabilistic shallow
  search.
- Use `RuleProfile` to decide which tactical features apply. If a rule disables
  or limits throw-ins, transfers, or attack-card count, the evaluator must not
  score unavailable pressure as if it existed.
- Update `docs/EVALUATION_MODEL.md` only after implementation, so that document
  keeps describing current behavior rather than planned behavior.

## Information Modes

Evaluation has two useful information modes. V2 should implement the fair mode
first and leave oracle mode for later.

`SeatViewFair` is the production mode. It may use the evaluated hand, public
table cards, visible trump card, visible hand sizes, stock count, discard cards,
public action history, and cards that the evaluated seat can infer with
certainty. It must not use hidden hands or unrevealed stock order.

`OracleReview` is a future training mode. It may use full replay data, hidden
hands, and exact stock order to explain what was actually possible. This is
useful for post-game teaching, but it should not drive normal offline bot play
or normal move labels until it is explicitly separated from fair evaluation.

## Card Memory

V2 should consume a card-memory layer between `DecisionContext` and the
evaluator. That layer belongs to the app/game surface, not to the evaluator
itself. The separate public-history iteration is specified in
`docs/2026-06-16-public-card-history-specs.md`.

The memory model should track:

- own hand;
- public table cards;
- visible trump indicator;
- discard cards;
- cards the opponent certainly took from the table;
- ranks and suits already seen;
- unknown cards that may still be in stock or opponent hands.

In live play, the runner currently gives controllers only the current
`TurnContext`. That is enough for stateless scoring, but not enough for durable
memory. V2 should use an app-level memory snapshot so that evaluators, stronger
bots, training views, and stored analysis do not reimplement the same tracking
logic.

Stored match analysis can reconstruct stronger memory from internal replay, but
it should still score from the acting seat's fair information unless explicitly
running oracle review.

## Belief Model

Card memory should feed a simple belief model. The first version can stay
count-based and deterministic.

For each opponent, estimate:

- likely trump count;
- likely rank availability for throw-ins;
- likely suit voids or suit weakness;
- whether known taken cards create dangerous future rank pairs;
- how sharply the unknown pool has collapsed as stock and discard grow.

The model does not need full Bayesian sampling in this iteration. It only needs
better features than the current `opponent hand size / unknown pool size`
estimate.

## Throw-In Pressure

Throw-in scoring should depend on defender state and phase.

When the defender is taking or has weak defense coverage, each extra throw-in is
usually valuable. It removes one card from the attacker and adds one card to the
defender, a two-card swing in card economy.

When the defender can probably beat everything, a throw-in may be bad. It can
help the defender discard cards and take initiative. The evaluator should reduce
throw-in value when defense coverage is strong and the thrown card is expensive.

Useful throw-in features:

- net card swing if the defender takes;
- probability that the defender can fully cover the table;
- rank multiplicity in own hand;
- known or likely opponent cards with the same ranks;
- cost of the thrown card, with high trump and high non-trumps penalized;
- stock phase, because card swings matter more when the stock is low or empty.

`done` should remain valuable when no useful pressure exists. It should stop
being the default best action after every successful defense.

## Defense Safety

V2 should score defense by asking what the defense leaves behind.

Important defense features:

- cheapest sufficient defense;
- trump spent versus non-trump spent;
- number of pending attacks after the move;
- ranks left on the table that enable opponent throw-ins;
- known or likely opponent cards matching those ranks;
- remaining defense coverage after playing the chosen card;
- endgame initiative if the defense succeeds.

Sometimes taking is better than defending with a critical trump and then taking
more cards on the next throw-in. The evaluator should allow that pattern,
especially early or midgame when taking can improve hand structure and the stock
still replenishes cards.

## Hand Shape

The evaluator should score hand structure, not only hand size.

Candidate features:

- trump count and rank-weighted trump strength;
- isolated low non-trumps that are good throw-in material;
- pairs and rank groups that create attack chains;
- short suits and void suits;
- high cards trapped in weak suits;
- known cards in opponent hand that make a retained rank dangerous or useful.

This makes the evaluator understand why giving the opponent unpaired, off-suit,
or hard-to-connect cards can be good late in the game.

## Rule Awareness

The scoring model must read rule configuration before applying tactical terms.

Examples:

- If throw-ins are restricted, lower throw-in pressure according to the actual
  eligible seats and attack limit.
- If transfers are disabled, remove transfer bonuses and transfer risk.
- If attack count is capped by defender hand size, do not score impossible
  overload sequences.
- If ordered throw-ins are introduced later, pressure should depend on whose
  throw-in opportunity comes next.

This belongs in the evaluator boundary, not in the domain state machine. The
domain remains responsible for legal actions. The evaluator explains and ranks
only the legal actions it receives.

## Implementation Slices

1. Implement the app-level public card-history snapshot. Done.
2. Feed fair memory into stored match analysis. Done.
3. Feed fair memory into live arena controllers. Done.
4. Add first belief features for known opponent cards. Done.
5. Rework `throw_in` versus `done` scoring around pressure. Done.
6. Add first defense safety feature for known throw-in cards. Done.
7. Add deeper rule-aware scoring inputs. Still pending beyond legal-action
   filtering.
8. Run mirrored 2-player benchmarks and document the results. Done.

The first two or three slices should avoid broad refactors. If the memory
surface becomes too large, stop and split the app/controller interface change
into its own small iteration.

## Acceptance Criteria

- `heuristic` beats `simple` in mirrored 2-player arena runs on 500+ matches by
  at least 55-60% excluding draws.
- The `heuristic` action profile contains meaningful throw-ins in two-player
  games instead of almost always choosing `done`.
- The analyzer no longer treats most successful pressure throw-ins as automatic
  inaccuracies.
- Unit tests cover throw-in pressure, defense safety, known taken cards, and
  rule-disabled tactics.
- `make check` passes.

Current benchmark result:

```text
heuristic vs simple, 1000 matches, seed 202: heuristic 464, simple 471, draws 65
simple vs heuristic, 1000 matches, seed 202: simple 485, heuristic 465, draws 50
```

The initial v2 work fixes the original under-throwing failure but leaves
`heuristic` much closer to `simple`, but still behind over the mirrored
2000-match sample. The next improvement should not be another broad weight
tweak. It should add a narrower search or endgame model that can exploit public
memory more directly.

## Out of Scope

- Full MCTS.
- Learned weights.
- Win probability.
- TUI or training UI.
- SSH, Wish, daemon integration.
- Oracle move labels in normal play.
- Perfect endgame solver, except as a possible later small-hand mode.

## Research Inputs

Public strategy and AI material supports the same direction: Durak rewards card
pressure, trump conservation, memory of played cards, rank-pair attacks, and
phase-aware endgame play.

- Wikipedia Durak rules: https://en.wikipedia.org/wiki/Durak
- Online Durak strategy notes:
  https://online-durak.com/en/strategy-of-game-durak-subtleties-and-nuances/
- Veniamin Ilmer two-player analysis:
  https://veniamin-ilmer.github.io/analysis/durak.html
- Azamat Zarlykov Durak AI thesis:
  https://dspace.cuni.cz/bitstream/handle/20.500.11956/179615/130351832.pdf?sequence=1&isAllowed=y
- Ilmari Vahteristo Durak AI thesis:
  https://lutpub.lut.fi/bitstream/handle/10024/165592/Vahteristo_Ilmari_kandidaatintyo.pdf?sequence=1
