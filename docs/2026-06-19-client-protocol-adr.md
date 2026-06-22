# ADR: Client Protocol Freeze

## Status

Accepted for the current milestone: do not freeze protobuf yet.

## Context

Epic 1 added `internal/app/client` as a local contract over `app.Series` and
`app.Session`. Epic 2 added `durak tui` on that contract.

The current proven contract covers local UI needs:

- render state;
- submit a current legal action id;
- advance controller turns;
- concede;
- start the next match.

It does not yet cover remote table lifecycle, player identity, reconnects,
spectators, auth, durable table ownership, transport errors, or concurrent
clients.

## Decision

Keep the Go client DTOs as the canonical contract for now. Do not add protobuf
or generated code in this milestone.

The next protocol decision should happen only when one of these becomes true:

- a daemon prototype needs a stable process boundary;
- a second non-Go frontend must consume the same contract;
- remote multiplayer needs explicit table/session lifecycle messages.

When that happens, freeze schema and transport separately. Protobuf can describe
messages, but it should not decide whether the transport is gRPC, SSH, HTTP, or
something else.

## Consequences

- CLI and TUI can keep moving without generated-code churn.
- The contract can still change cheaply while TUI usage exposes missing fields.
- The daemon is not blocked, but its first task should define table lifecycle
  before schema generation.
- React or another frontend remains possible later, but it is not a reason to
  freeze transport now.

## Rejected Options

### Generate protobuf now

Rejected because the only proven consumers are in-process Go clients. Protobuf
would encode gaps we have not modeled yet, especially table lifecycle and
reconnect behavior.

### Start daemon before protocol shape

Rejected because the daemon would force identity, concurrency, persistence, and
transport decisions before the local client flow has stabilized.

### Route CLI and TUI through a local daemon immediately

Rejected because it adds process management and transport failure modes without
current product value.

## Next Step

Continue with local client/TUI refinement or a minimal table-registry design.
Do not create `api/durak/v1/*.proto` until this ADR is revisited with a concrete
remote-client requirement.
