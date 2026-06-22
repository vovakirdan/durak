# Client Protocol and TUI Path Task Breakdown

**Goal:** Move from the CLI MVP to a TUI-ready client boundary without committing
to daemon networking or protobuf before the UI contract is proven.

**Approach:** Add a small local client-facing layer over `app.Series` and
`app.Session`, then build the TUI on that layer. Keep the first contract as Go
DTOs and action ids; freeze protobuf only after the TUI proves the shape. The
daemon follows the same contract later instead of driving this iteration.

**Skills:** brainstorming, task-breakdown, writing-clearly-and-concisely, Go
project rules from `docs/PROJECT_RULES.md`.

**Tech Details:** Go 1.26.1, `internal/domain`, `internal/app`,
`internal/adapters/cli`, future `internal/adapters/tui`, `cmd/durak`, Bubble Tea
when Epic 2 starts, protobuf only after Epic 3 is accepted.

---

## Direction

The next product step is TUI through a protocol-shaped local client contract.
This keeps the TUI from owning game state and leaves room for a daemon, protobuf,
or a future React client without forcing those choices now.

Do not start with a daemon. A daemon adds identity, table lifecycle, reconnects,
concurrency, persistence semantics, transport errors, and operator concerns
before the client shape is proven.

Do not start with protobuf. Protobuf should encode a known contract, not create
one. The first contract lives as Go DTOs with primitive fields and stable action
ids.

## Epics

1. **Client Contract:** Create a local client-facing contract and facade over the
   existing app/session layer.
2. **TUI Prototype:** Build a Bubble Tea TUI against that contract.
3. **Protocol Freeze:** Convert the proven contract into a protobuf/API spec only
   after the TUI has exercised it.
4. **Daemon Prototype:** Implement the first daemon backend for the frozen
   contract.
5. **Remote Multiplayer:** Add SSH or remote table flows after the daemon has a
   working local contract.

## Non-Goals

- No heuristic/evaluator changes.
- No React UI in this plan.
- No protobuf generator in Epic 1 or Epic 2.
- No daemon before the TUI contract is exercised locally.
- No CLI rewrite unless the TUI contract exposes a real shared need.

## Epic 1: Client Contract

**Status:** Implemented in `internal/app/client`. Contract details are tracked
in `docs/2026-06-19-client-contract-specs.md`; user-story status is tracked in
`docs/feature-status.csv`.

### Task 1: Add Client DTOs

**Files:**

- Create: `internal/app/client/state.go`
- Create: `internal/app/client/state_test.go`
- Reference: `internal/app/view.go`
- Reference: `internal/domain/action.go`

**Steps:**

1. Write a failing test that builds an `app.DecisionContext` with a hand, table,
   and legal actions, then projects it to a client `State`.
2. Assert the state uses primitive, transport-friendly values: card codes,
   integer seats, string phases, labels, and action ids.
3. Add `State`, `Card`, `TablePair`, and `LegalAction` structs.
4. Add `StateFromDecision(matchID, version, decision)` or equivalent.
5. Use action ids based on the current legal-action order, such as `"1"`,
   `"2"`, and `"3"`.
6. Keep domain structs out of exported DTO fields.
7. Run:

```sh
go test ./internal/app/client
```

Expected: tests pass.

### Task 2: Add Local Game Facade

**Files:**

- Create: `internal/app/client/local.go`
- Create: `internal/app/client/local_test.go`
- Reference: `internal/app/series.go`
- Reference: `internal/app/session.go`
- Reference: `internal/app/player.go`

**Steps:**

1. Write a failing test that creates a local game for one human seat and one
   controller seat.
2. Add a concrete `LocalGame` type. Do not add an interface yet.
3. Store the current `*app.Session`, human seat, match id, controllers, and a
   local state version.
4. Add `State() State`.
5. Add `SubmitAction(ctx, actionID string) (State, error)` that maps the id back
   to the current legal action and applies it through `Session.ApplyAction`.
6. Return a typed error for unknown action ids.
7. Increment the local state version only after accepted actions.
8. Run:

```sh
go test ./internal/app/client
```

Expected: tests pass.

### Task 3: Advance Controller Turns

**Files:**

- Modify: `internal/app/client/local.go`
- Modify: `internal/app/client/local_test.go`
- Reference: `internal/adapters/cli/game.go`
- Reference: `internal/app/runner.go`

**Steps:**

1. Write a failing test where the current active seat is controlled by a bot.
2. Add `Advance(ctx) (State, error)` to play controller turns until the human
   seat has legal actions or the match is complete.
3. Reuse the same polling behavior as CLI/arena: attacker first for attacks and
   throw-ins, defender first for defense.
4. Submit controller decisions through `Session.ApplyPlayerDecision`.
5. Return an error if no playable controller seat exists.
6. Run:

```sh
go test ./internal/app/client
```

Expected: tests pass.

### Task 4: Support Concede and Next Match

**Files:**

- Modify: `internal/app/client/local.go`
- Modify: `internal/app/client/local_test.go`

**Steps:**

1. Write a failing test for `Concede(ctx)` from the human seat.
2. Add `Concede(ctx) (State, error)`.
3. Write a failing test for starting the next match after completion.
4. Add `NextMatch(ctx) (State, error)` using the existing `app.Series` flow.
5. Preserve the "next match starts before previous loser" behavior through
   `Series.CompleteMatch` and `Series.StartMatch`.
6. Run:

```sh
go test ./internal/app/client
```

Expected: tests pass.

### Task 5: Document the Contract

**Files:**

- Create: `docs/2026-06-19-client-contract-specs.md`
- Modify: `README.md`

**Steps:**

1. Document the local client contract: state, legal actions, submit action,
   advance, concede, and next match.
2. State that this is not the protobuf schema yet.
3. State that protobuf starts only after the TUI proves the state/action shape.
4. Add the spec to the README planning-doc list.

## Epic 2: TUI Prototype

**Status:** Implemented. Tasks 6-10 have the first local Bubble Tea prototype
under `internal/adapters/tui` and `durak tui`; visual polish is intentionally
out of scope until the contract is exercised.

### Task 6: Add TUI Entrypoint

**Files:**

- Create: `cmd/durak/tui.go`
- Modify: `cmd/durak/main.go`
- Create: `internal/adapters/tui/model.go`
- Create: `internal/adapters/tui/model_test.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Steps:**

1. Before implementation, fetch current Bubble Tea docs with the project-approved
   documentation tool.
2. Add the smallest Bubble Tea dependencies needed for a basic model.
3. Add `durak tui` as a subcommand. Keep default CLI play unchanged.
4. Add a TUI model that owns only presentation state and a `client.LocalGame`.
5. Add a smoke test for model initialization without starting an interactive
   terminal.
6. Run:

```sh
go test ./cmd/durak ./internal/adapters/tui
```

Expected: tests pass.

### Task 7: Render Playable State

**Files:**

- Modify: `internal/adapters/tui/model.go`
- Create: `internal/adapters/tui/view_test.go`

**Steps:**

1. Write a failing render test for a known `client.State`.
2. Render phase, seats, trump, table, hand, stock, discard, and legal actions.
3. Keep text dense and readable. Do not add decorative layout work.
4. Show action ids exactly as the client contract provides them.
5. Run:

```sh
go test ./internal/adapters/tui
```

Expected: tests pass.

### Task 8: Submit Actions from TUI

**Files:**

- Modify: `internal/adapters/tui/model.go`
- Modify: `internal/adapters/tui/model_test.go`

**Steps:**

1. Write a failing update test where pressing a number selects an action id.
2. Add number-key handling for visible legal actions.
3. Submit the selected action through `client.LocalGame.SubmitAction`.
4. Call `Advance` after accepted human actions.
5. Show errors in the model state instead of panicking.
6. Run:

```sh
go test ./internal/adapters/tui
```

Expected: tests pass.

### Task 9: Finish Match Flow

**Files:**

- Modify: `internal/adapters/tui/model.go`
- Modify: `internal/adapters/tui/model_test.go`

**Steps:**

1. Write a failing update test for completed-match state.
2. Add `n` for next match and `q` for quit.
3. Wire `n` to `client.LocalGame.NextMatch`.
4. Keep match result rendering in the normal state view.
5. Run:

```sh
go test ./internal/adapters/tui
```

Expected: tests pass.

### Task 10: TUI Manual Smoke

**Files:**

- Modify: `README.md`

**Steps:**

1. Add a short README command:

```sh
go run ./cmd/durak tui -seed 42 -bot simple
```

2. Run:

```sh
make check
go run ./cmd/durak tui -seed 42 -bot simple
```

Expected: checks pass and a local TUI game can submit at least one legal action.

## Epic 3: Protocol Freeze

**Status:** ADR accepted. Protobuf is deferred until a daemon process boundary
or non-Go frontend makes the stable transport schema necessary.

### Task 11: Write Protocol ADR

**Files:**

- Create: `docs/2026-06-19-client-protocol-adr.md`

**Steps:**

1. Compare the TUI-used DTO contract against likely CLI, daemon, and React needs.
2. Decide whether remote clients need protobuf now.
3. If yes, pick the transport separately from the schema.
4. Record rejected choices and why they stay rejected.

### Task 12: Add Proto Only After ADR Approval

**Files:**

- Create only after approval: `api/durak/v1/*.proto`
- Create only after approval: generated Go files or generation scripts
- Modify only after approval: `go.mod`
- Modify only after approval: `go.sum`

**Steps:**

1. Add proto messages that mirror the proven client DTOs.
2. Add generated Go types or a documented generation command.
3. Add mapper tests between proto messages and client DTOs.
4. Do not expose domain structs through proto.
5. Run:

```sh
go test ./...
```

Expected: tests pass.

## Epic 4: Daemon Prototype

**Status:** Implemented through the first local daemon command. Task 13 has a
minimal in-memory table registry around `client.LocalGame`; Task 14 adds
`durakd status` without network transport.

### Task 13: Add Local Table Registry

**Files:**

- Create: `internal/app/server/registry.go`
- Create: `internal/app/server/registry_test.go`

**Steps:**

1. Add an in-memory table registry around `client.LocalGame`.
2. Use a single mutex per table for mutation.
3. Add tests for create table, join seat, submit action, and stale action ids.
4. Keep persistence out of this task.

### Task 14: Add First Daemon Command

**Files:**

- Create: `cmd/durakd/main.go`
- Create: `cmd/durakd/main_test.go`

**Steps:**

1. Add a daemon binary only after the registry tests pass.
2. Start with a local-only development mode.
3. Expose one health/status command or endpoint.
4. Do not add auth until a remote transport exists.

Current implementation: `go run ./cmd/durakd status`.

## Epic 5: Remote Multiplayer

### Task 15: Choose SSH or API Transport

**Files:**

- Modify: `docs/ARCHITECTURE.md`
- Modify: `docs/STACK.md`

**Steps:**

1. Use the protocol ADR and daemon prototype to choose the next transport.
2. If terminal multiplayer is still the target, prefer Wish-hosted TUI first.
3. If non-terminal clients become real, add an API transport for the frozen proto.
4. Do not build both transports in the same milestone.

## Execution Rules

- Finish Epic 1 before starting TUI.
- Finish TUI before freezing protobuf.
- Freeze protobuf before writing a remote daemon API.
- Keep all game mutations on the existing app/domain validation path.
- Keep heuristic/evaluator work out of these epics.
- Use `make check` at the end of each epic.
