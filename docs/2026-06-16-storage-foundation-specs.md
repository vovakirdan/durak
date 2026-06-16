# Storage Foundation Spec

Status: Implemented for the current SQLite foundation iteration.

## Purpose

The next persistence milestone should create a durable SQLite foundation for
match history, replay, future scoring, future rating, future currency, and AI
training/export. This iteration must not implement scoring, rating, player
accounts, betting, or the economy. Its job is to make the event/history storage
shape correct enough that those systems can be added without rewriting match
persistence.

The canonical durable source of truth is the internal match event stream. Public
events are a safe projection/export. Summaries are read models. Config snapshots
are immutable records that explain which app-level rules produced a match.

## Current Implementation

- `internal/app.MatchRecorder` accepts one transactional batch with the config
  snapshot, public events, internal events, and an optional completed-match
  summary.
- `internal/adapters/storage.SQLiteStore` applies embedded Goose migrations and
  stores the batch with Bun inside a SQLite transaction.
- `cmd/durak -db` and `cmd/durak arena -db` write durable match history.
- `cmd/durak history -db` lists completed-match summaries.
- `cmd/durak replay -db -match-id <id>` reconstructs a stored match from
  canonical internal events.
- JSONL remains a public event export/debug adapter. It is not the canonical
  replay source because it omits hidden deal state.

## Library Choices

- SQLite is the first indexed durable store.
- Bun is the runtime database framework and query layer.
- Goose is the migration runner and schema versioning tool.
- Bun's SQLite shim and SQLite dialect are the preferred SQLite path for a
  portable binary.

Dependencies should be installed with `go get <module>@latest`, followed by
`go mod tidy`.

Planned packages:

- `github.com/uptrace/bun`
- `github.com/uptrace/bun/dialect/sqlitedialect`
- `github.com/uptrace/bun/driver/sqliteshim`
- `github.com/pressly/goose/v3`

Runtime storage code should use Bun models, transactions, and query builders.
Raw SQL strings are not allowed in runtime storage code without a narrow,
documented exception. Goose SQL migration files are allowed because migrations
are the explicit schema contract rather than runtime business-query code.

## Why Not Other Options

`sqlc` is a strong Go persistence tool, but it requires writing SQL queries as
the primary query interface. That conflicts with the current no-runtime-raw-SQL
direction.

Ent is a strong type-safe ORM, but it brings code generation and a heavier schema
toolchain. The project already has a strict source-file size rule and Sentrux
reports modularity as the main quality bottleneck, so this iteration should avoid
generated ORM layers unless persistence complexity proves they are needed.

GORM is broad and convenient, but its higher-level behavior is less attractive
for event streams and ledger-like future storage where explicit transactions and
projection boundaries matter.

## Scope

In scope:

- Add embedded Goose migrations.
- Add a SQLite storage adapter under `internal/adapters/storage`.
- Add a single app-level transactional recording port for durable storage.
- Persist immutable match config snapshots.
- Persist match metadata.
- Persist canonical internal event envelopes.
- Persist public event envelopes as a projection/export.
- Persist completed match summaries derived from event streams.
- Read public events by match.
- Read internal events by match for replay.
- List match summaries for history.
- Keep the existing JSONL store as a local debug/export adapter.

Out of scope:

- Player accounts and identity.
- Rating calculation.
- Currency balances, grants, stakes, purchases, or ledger effects.
- Save/resume from arbitrary mid-match snapshot.
- TUI or SSH daemon integration.
- AI trace storage in SQLite.
- External file/YAML/JSON config format.

## Schema Shape

`match_config_snapshots`:

- `hash` as the primary identity, matching `MatchConfigIdentity.Hash`;
- `schema_version`;
- `rule_preset`;
- `rule_profile`;
- `config_json`;
- `created_at`.

`matches`:

- `match_id` primary key;
- optional `series_id`;
- `config_hash` foreign key;
- `player_count`;
- `status`;
- `started_at`;
- optional `completed_at`;
- `last_sequence`.

`match_internal_events`:

- `match_id`;
- `sequence`;
- `kind`;
- `envelope_schema_version`;
- `payload_json` or `envelope_json`;
- `created_at`;
- primary key on `(match_id, sequence)`.

`match_public_events`:

- same identity/sequence shape as internal events;
- stores only public event envelopes;
- primary key on `(match_id, sequence)`.

`match_summaries`:

- `match_id` primary key;
- `rule_profile`;
- `config_hash`;
- seat data as JSON arrays where the app model is already slice-based;
- trump, first attacker, defender, initial stock count, action count;
- completion fields: completed, winner, loser, draw, conceded_by;
- `last_sequence`;
- timestamps for projection writes.

Exact column names can be refined during implementation, but the model must keep
seat-based fields and must not regress to player1/player2 assumptions.

## Application Boundary

The current `EventStore` and `InternalEventStore` ports are intentionally
independent. They are good for tests and JSONL, but not enough for durable DB
semantics. SQLite storage should be wired through a new transactional app port.

The new port should accept a single batch that can contain:

- the config snapshot for the match start;
- public events;
- internal events;
- an optional match summary when a match completes.

The app layer should call this as one durable operation so the SQLite adapter can
write all relevant rows inside one transaction. If a transaction fails, domain
events must stay pending in the active session just like current event-store
failures do.

Existing in-memory and JSONL stores should remain available. JSONL can continue
to implement public event export without becoming canonical storage.

## Transaction and Failure Semantics

SQLite writes for a batch must be transactional. A batch must never leave only
the internal event without the corresponding public projection, or only the
summary without the final event that produced it.

The primary uniqueness boundary is `(match_id, sequence)`. Duplicate or
conflicting writes should return typed storage errors. Full idempotent retry
semantics can be refined before daemon mode, but the schema and adapter should
not make idempotency impossible.

Context cancellation must stop storage work before commit when possible. Storage
errors should be wrapped in adapter-level typed errors, not leaked as raw driver
details through CLI messages.

## Read Models and Future Scoring

`match_summaries` is the first durable projection. It should be rebuildable from
event streams and should stay separate from future scoring/rating tables.

Future scoring can read internal events, public events, summaries, and config
snapshots. It should produce separate score/evaluation records rather than
mutating source events. Rating and currency should later be ledger-like and
transactional with match completion effects.

## Testing Requirements

The storage iteration should include tests for:

- running migrations on a fresh in-memory or temp-file SQLite database;
- appending and reading public events;
- appending and reading internal events for replay;
- persisting and reading a config snapshot;
- persisting and reading match summaries;
- rolling back a transaction when any part of a batch fails;
- rejecting duplicate/conflicting sequences;
- keeping JSONL behavior unchanged.

Project verification remains `make check`. For storage changes, `go test -race
./...` is recommended before commit because storage adapters introduce shared
resources and transactions.
