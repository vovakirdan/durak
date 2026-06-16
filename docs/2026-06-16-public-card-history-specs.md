# Public Card History Spec

Status: initial runtime implementation added.

## Purpose

Public card history should model what a real player could remember during a
match. It is runtime memory, not a SQL persistence feature. It should not reveal
hidden hands, hidden stock order, or exact drawn cards from the stock. It should
give future evaluators, bots, training views, and review tools one shared source
of fair card memory.

This is a separate iteration because card memory is broader than the evaluator.
The evaluator needs it for better scoring, but humans, training overlays, bot
difficulty, and post-game review will need the same facts later.

## Layer Decision

Public card history belongs in the application layer as a seat-scoped read
model. The domain core should stay focused on legal actions, state transitions,
and events. The evaluator should consume memory snapshots; it should not become
the owner of match-history reconstruction.

The current implementation keeps the read model in process with the active
match/session. A future hosted daemon may move runtime memory to Redis or another
ephemeral store, but this feature should not create SQLite tables or durable
memory snapshots.

This avoids duplicate logic:

- the evaluator can score positions from a fair memory snapshot;
- future bots can use memory without copying evaluator internals;
- training and review screens can display or hide memory aids;
- arena and stored analysis can use the same reconstruction path.

## Fair Information Boundary

The history model may record only facts that were visible at the table or can be
deduced from visible facts.

Allowed:

- cards in the evaluated seat's hand;
- cards played to the table;
- the visible trump indicator;
- cards moved to discard;
- cards a defender took from the table;
- who played each visible card;
- hand-size changes and stock-count changes;
- public rule configuration that affects legal pressure.

Forbidden:

- hidden opponent hands;
- exact stock order;
- exact cards drawn from stock;
- internal deal data, except when rendering an explicit future oracle mode;
- any future anti-cheat or cheating-related signal.

When a defender takes, every card on the table becomes known to be in that
defender's hand until it is later played or discarded. When a player refills
from stock, only the draw count is public. The specific drawn cards remain in
the unknown pool.

## Consumers

The first consumers are internal:

- `evaluation`: improve hidden-card and belief features;
- `arena`: score live bot decisions with fair memory;
- `analyze`: replay stored matches through the same fair memory model.

Later consumers can be added without changing the core model:

- `heuristic` and future stronger offline bots;
- optional training overlays;
- post-game review screens;
- difficulty settings that decide how much memory a bot may use.

`simple` does not need this model. It can remain a deterministic baseline that
uses only the current legal-action list.

## Data Shape

The first model is explicit and small.

Suggested snapshot fields:

- evaluated seat;
- known own hand;
- known table cards;
- known discard cards;
- known cards currently in each opponent hand, inferred from take events;
- seen card set;
- unknown card pool;
- per-rank seen counts;
- per-suit seen counts;
- public hand sizes;
- stock count;
- confidence score.

Known opponent cards are not oracle data. They are cards the opponent visibly
took from the table and has not visibly played away.

## Event Reconstruction

The model updates from public events where possible. Stored internal
events may be used as a replay source only to reconstruct the public sequence;
the memory builder must ignore hidden fields such as initial hidden hands and
stock order in fair mode.

Required updates:

- `attack`, `defend`, `throw_in`, and `transfer` mark played cards as seen and
  remove those cards from the acting seat's known-held set if present;
- `take` and `finish_take` keep the current table in a pending taken set;
- `round_ended` with a take outcome moves table cards into the defender's
  known-held set;
- `round_ended` with successful defense moves table cards into discard;
- `refill` updates only hand sizes and stock count, not card identities.

Exact event names can follow the existing app/domain event names during
implementation. The behavioral boundary matters more than the final type names.

## Turn Context Integration

The app layer exposes memory as an optional seat-scoped snapshot.

The current implementation adds `PublicMemory` to `DecisionContext`. Existing
controllers continue to work unchanged because the field is optional for
callers that do not use it.

This keeps the baseline bots simple while making stronger bots and training
features possible.

## Acceptance Criteria

- Public memory reconstructs known taken cards without hidden deal or stock
  data.
- Refill events never reveal exact drawn cards in fair mode.
- The evaluator can consume the memory snapshot without replaying events itself.
- Stored analysis and live arena can use the same memory model.
- Tests cover taking, successful defense, later play of a known taken card, and
  refill without card identity.
- `make check` passes.

## Out of Scope

- Oracle review.
- UI display of memory aids.
- Bot difficulty settings.
- Persistent analytics tables.
- SQL storage for memory snapshots.
- Anti-cheat, cheating detection, or hidden-state inspection.
- Full probabilistic shallow search.
