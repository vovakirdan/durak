# Client Contract Specs

## Status

Implemented in Epic 1 as an in-process Go contract under `internal/app/client`.
This is not the protobuf schema yet. Protobuf should encode this contract only
after a TUI has exercised the state and action shape.

## Purpose

The client contract gives CLI/TUI/future daemon clients one narrow surface:

- render a client `State`;
- submit a legal action by stable action id for that state;
- advance controller turns until the local human can act;
- concede the current match;
- start the next match after completion.

The contract intentionally sits over `app.Series` and `app.Session`. It does not
own domain rules, controller strategy, replay, storage, or daemon lifecycle.

## State

`State` is transport-friendly. Exported fields use strings, integers, counters,
and DTO slices instead of `domain` structs or enum types.

Core fields:

- `MatchID`, `Version`, `Seat`;
- `Phase`, `Attacker`, `Defender`;
- `TrumpSuit`, `TrumpIndicator`;
- `Table`, `Hand`, `HandSizes`;
- `StockCount`, `DiscardCount`, `SuccessfulDefenses`;
- `Winner`, `Loser`, `Result`;
- `LegalActions`.

Cards expose:

- `Code`: compact card code such as `6C` or `AH`;
- `Rank`: rank code;
- `Suit`: suit code.

Legal actions expose:

- `ID`: one-based action id derived from the current legal-action order;
- `Kind`: stable string kind such as `attack`, `defend`, `take`;
- `Label`: renderable command label;
- `Card`: optional card DTO;
- `AttackIndex`: zero-based attack index for defense actions.

Action ids are valid only for the current state/version. Clients should refresh
state after every accepted command and never cache ids across turns.

## Local Game Facade

`LocalGame` is the first implementation of the contract.

Construction:

```go
game, err := client.NewLocalGame(ctx, client.LocalGameOptions{
    SeriesID:    "series-1",
    BaseMatchID: "match-1",
    PlayerCount: 2,
    HumanSeat:   0,
    Controllers: controllers,
})
```

Methods:

- `State() State` returns the human-seat state.
- `SubmitAction(ctx, actionID) (State, error)` maps the id to the current legal
  action and applies it through `Session.ApplyAction`.
- `Advance(ctx) (State, error)` runs controller turns until the human has legal
  actions or the match completes.
- `Concede(ctx) (State, error)` concedes from the human seat.
- `NextMatch(ctx) (State, error)` records the completed match in `Series` and
  starts the next consecutive match.

Typed client errors:

- `ErrUnknownActionID`;
- `ErrMatchInProgress`;
- `ErrNoActiveController`;
- `ErrInvalidLocalGame`.

Existing app errors still pass through for domain or controller failures, such
as `app.ErrMissingPlayerController`, `app.ErrIllegalAction`, and
`app.ErrActionLimitExceeded`.

## Flow

1. Client calls `State`.
2. If `LegalActions` is non-empty, render action ids and wait for human input.
3. Client calls `SubmitAction` with the selected id.
4. Client calls `Advance` to let controllers act.
5. Repeat until `Phase == "complete"`.
6. Client calls `NextMatch` or exits.

## Non-Goals

- No daemon lifecycle.
- No protobuf generator.
- No public network protocol.
- No storage or replay API.
- No heuristic or evaluator tuning.

## Verification

```sh
go test ./internal/app/client
```
