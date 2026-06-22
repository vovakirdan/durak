# Seat-Aware SSH Tables Task Breakdown

**Goal:** Turn the current shared SSH table from one replayable local-human game
into an in-memory table that can serve multiple human seats over separate SSH
TUI sessions.

**Approach:** Keep the TUI contract unchanged and move multi-seat ownership into
`internal/app/server`. A table owns `app.Series` and `app.Session` directly,
projects `client.State` per seat, serializes mutations, and lets each SSH
session attach to one seat.

**Skills:** brainstorming, task-breakdown, coding, commit-work.

**Tech Details:** Go 1.26.1, `internal/app`, `internal/app/client` DTOs,
`internal/app/server`, `internal/adapters/tui`, `internal/adapters/ssh`,
Wish-hosted Bubble Tea sessions.

---

## Direction

This is the next product-facing step after `durakd ssh -table`. The previous
shared table proved that multiple SSH sessions can attach to one daemon-owned
game, but it still used `client.LocalGame`, so every session viewed and acted as
the same human seat.

The next useful capability is true seat-aware table play:

- `durakd ssh -table demo` creates one in-memory table for the daemon process.
- `ssh localhost -p 23234 seat 0` attaches to seat 0.
- `ssh localhost -p 23234 seat 1` attaches to seat 1.
- Each session renders that seat's hand and legal actions.
- Table mutations stay serialized through the app/session validation path.

## Non-Goals

- No protobuf schema in this PR.
- No durable table persistence.
- No auth, reconnect identity, or spectators.
- No evaluator or heuristic changes.
- No React or HTTP API.

## Tasks

### Task 1: Add Seat-Aware Table Core

**Files:**

- Create or modify: `internal/app/server/registry.go`
- Create or modify: `internal/app/server/table_game.go`
- Modify: `internal/app/server/registry_test.go`

**Steps:**

1. Replace the registry table internals with an app-level table that owns
   `app.Series` and `app.Session` directly.
2. Project state with `client.StateFromDecision(matchID, version, decision)`.
3. Add `State(seat)`, `SubmitAction(seat, version, actionID)`, `Concede(seat)`,
   and `NextMatch(seat)`.
4. Keep action ids scoped to the displayed seat state.
5. Advance only controller-owned seats; stop when a human seat can act or the
   match is complete.
6. Preserve a per-table mutex around all mutation.

### Task 2: Add Seat Lifecycle

**Files:**

- Modify: `internal/app/server/registry.go`
- Modify: `internal/app/server/table_game.go`
- Modify: `internal/app/server/registry_test.go`

**Steps:**

1. Define human seats for a table.
2. Allow joining only configured human seats.
3. Reject a second live session for an occupied seat.
4. Release the seat when the TUI game is closed.
5. Leave reconnect and identity out of scope.

### Task 3: Wire SSH Seat Selection

**Files:**

- Modify: `internal/adapters/ssh/server.go`
- Modify: `internal/adapters/ssh/server_test.go`
- Modify: `cmd/durakd/main.go`
- Modify: `cmd/durakd/main_test.go`

**Steps:**

1. Parse SSH command arguments for `seat <n>`.
2. Default to seat 0 when no command is provided.
3. In `-table` mode, create a two-human-seat table by default.
4. Keep non-table SSH sessions as one local human vs bot games.
5. Return startup errors through the existing TUI error model.

### Task 4: Update Documentation and Status

**Files:**

- Modify: `README.md`
- Modify: `docs/2026-06-19-client-protocol-tui-tasks.md`
- Modify: `docs/feature-status.csv`

**Steps:**

1. Document `ssh localhost -p 23234 seat 0` and `seat 1`.
2. Add a feature-status row for seat-aware SSH tables.
3. Mark this as the continuation after Task 18 rather than a protocol freeze.

### Task 5: Verification and PR

**Steps:**

1. Run focused tests:

```sh
go test ./internal/app/server ./internal/adapters/ssh ./cmd/durakd
```

2. Run full checks:

```sh
make check
```

3. Smoke a local SSH table with two seats if feasible in this environment.
4. Commit as multiple logical commits and open one PR.
