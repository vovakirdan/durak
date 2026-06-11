# Architecture

## 1. Context

- **Product shape:** terminal-first Durak game. The MVP is local CLI play against a bot. The target system adds Bubble Tea TUI, Wish SSH hosting, multiplayer, persistent match history, ratings, internal currency, strategy DSL, and AI-backed bot decisions.
- **Relevant truth sources:**
  - `docs/2026-06-10-durak-prd.md`
  - `docs/STACK.md`
- **Key constraints from PRD and stack:**
  - Go is the primary stack.
  - Game rules must not depend on CLI, TUI, SSH, storage, or AI packages.
  - First implementation stays small: local CLI, one human seat, one bot seat.
  - Target model must support configurable rules, `seats[]`, persistent history, and SSH sessions.

## 2. Architectural Style

- **Chosen style:** modular monolith with ports/adapters around a framework-free game core.
- **Why this style fits the MVP:** one Go module and one local CLI binary are enough for the first playable game, while internal boundaries preserve the later path to TUI, Wish daemon, persistence, and AI strategy adapters.
- **Rejected alternatives:**
  - **TUI-first architecture:** fast for a visual prototype, but it risks placing game state inside Bubble Tea model state.
  - **Server-first architecture:** useful later for SSH multiplayer, but it would add identity, sessions, storage, and operations before the core game is proven.
  - **Microservices:** no scaling or team boundary requires it.

## 3. System Components

- **Domain core:** cards, deck, rule profile, match state, legal actions, state transitions, events, and outcome detection.
- **Application/session layer:** coordinates a match, accepts player actions, invokes bot strategies, owns active in-memory session state, and exposes snapshots to adapters.
- **CLI adapter:** parses terminal commands, renders text output, and calls the application layer.
- **Bot adapter:** implements strategy interfaces using read-only decision contexts.
- **Future TUI adapter:** Bubble Tea presentation and input layer over the same application/session layer.
- **Future SSH adapter:** Wish server/session bridge that hosts TUI sessions remotely.
- **Future persistence adapter:** event/snapshot storage, match history, ratings, and currency ledgers.

## 4. Interaction Surfaces

### Local CLI

- **Type:** CLI.
- **Primary responsibilities:** start a game, show visible state, parse commands, print validation errors, and drive the local human-vs-bot loop.
- **Internal structure:** command parser, renderer, and loop runner.
- **Key dependencies:** application/session layer only.

### Future Bubble Tea TUI

- **Type:** TUI.
- **Primary responsibilities:** richer terminal rendering, keyboard navigation, action selection, and game state updates.
- **Internal structure:** Bubble Tea model, update handlers, view renderer, and domain-to-view mapping.
- **Key dependencies:** application/session layer and Charm UI packages. It must not own rule logic.

### Future Wish SSH Surface

- **Type:** SSH-hosted terminal application.
- **Primary responsibilities:** create remote terminal sessions, map SSH identity to player identity, attach users to tables, and host a Bubble Tea program per session.
- **Internal structure:** Wish server setup, auth/session middleware, table/session registry, and TUI program factory.
- **Key dependencies:** application/session layer, future persistence adapter, and Charm Wish.

## 5. Execution and Service Components

### Domain Core

- **Responsibility:** deterministic game rules and state transitions.
- **Internal module structure:**
  - cards and deck types.
  - rule profile and built-in presets.
  - match state and round state.
  - action types and validation.
  - event types and visibility metadata.
  - deterministic shuffle/deal helpers using injected randomness.
- **Key dependencies:** Go standard library only.
- **Why this boundary exists:** it keeps the most important logic testable and reusable across CLI, TUI, SSH, bots, and future storage.

### Application/Session Layer

- **Responsibility:** orchestrate active matches and expose a stable API for adapters.
- **Internal module structure:**
  - session service.
  - player/seat registry for the active match.
  - command handling: submit action, ask bot, advance state.
  - snapshot/query methods for rendering.
  - event store port.
- **Key dependencies:** domain core, bot strategy interfaces, optional event store.
- **Why this boundary exists:** UI adapters should not mutate match state directly.

### Bot Strategy Layer

- **Responsibility:** choose actions for bot-controlled seats.
- **Internal module structure:**
  - strategy interface.
  - read-only decision context.
  - simple deterministic strategy for MVP.
  - future DSL and AI strategy adapters.
- **Key dependencies:** domain action/value types only.
- **Why this boundary exists:** bot decisions must be replaceable and validated through the same path as human actions.

### Future Persistence Layer

- **Responsibility:** durable match history, replay/training data, player profiles, rating state, and currency ledger.
- **Internal module structure:**
  - event store.
  - snapshot store if replay from full event history becomes expensive.
  - repositories for players, ratings, and currency.
  - migration runner.
- **Key dependencies:** storage library chosen at persistence milestone.
- **Why this boundary exists:** persistence should consume stable events and application commands without owning game rules.

## 6. Interfaces and Boundaries

- **Primary contracts/interfaces:**
  - `SessionService`: starts matches, accepts actions, advances bot turns, returns render snapshots.
  - `Strategy`: receives decision context and returns a proposed action.
  - `EventStore`: appends structured domain/application events for one match stream.
  - `RandomSource`: enables deterministic tests for shuffle and first-attacker fallback.
  - Future `PlayerStore`, `MatchStore`, `RatingStore`, and `CurrencyLedger`.
- **How parts communicate:**
  - CLI/TUI/SSH call application services.
  - Application services call domain core.
  - Bot strategies return proposed domain actions.
  - Persistence consumes application event streams and stores snapshots/records later.
- **API/event/schema strategy:**
  - In-process Go interfaces for MVP.
  - Application events include match id and sequence number around structured domain event payloads.
  - Stable serialized events use a JSON envelope: `schema_version`, `match_id`, `sequence`, `kind`, `visibility`, and `payload`.
  - Current serialized events are `visibility=public`; future private events must be explicit rather than mixed into public replay data.
  - Durable adapters may add record metadata such as insertion timestamp outside the envelope when a real store is introduced.
- **Compatibility notes across parts:**
  - Domain core imports no adapter packages.
  - Adapters may depend inward on application/domain packages.
  - Bot and human actions share the same validation path.
  - Future SSH concurrency must serialize mutations per match.

## 7. Data and State Architecture

- **Core entities and ownership:**
  - Domain core owns card, deck, rule, match, round, table, action, and event semantics.
  - Application/session layer owns active in-memory match sessions.
  - Future persistence owns durable records, not live rule execution.
- **Primary storage responsibilities:**
  - MVP: no durable storage.
  - First persistence milestone: append match events and store completed match summaries.
  - Later milestones: player profile, rating records, and currency ledger entries.
- **Transaction boundaries:**
  - MVP: no database transactions.
  - Future match completion transaction should persist final match events, summary, rating update, and currency ledger effects consistently.
- **Caching or queue usage:** none for MVP. Do not introduce cache or queue until daemon operations create a concrete need.

## 8. Async and Background Processing

- **What stays synchronous:**
  - Local CLI game loop.
  - Domain validation and state transitions.
  - MVP bot decisions.
- **What goes async:**
  - Nothing in MVP.
  - Future daemon may process each active match through a serialized command queue to avoid concurrent mutation from multiple SSH sessions.
- **Worker or job model:** no separate worker until there is scheduled economy work, analytics export, or AI training/export.
- **Retry, timeout, cancellation, and idempotency expectations:**
  - Human actions are not retried automatically.
  - Future AI decisions must have timeout/fallback behavior.
  - Future persistence writes should be idempotent by match id and event sequence.

## 9. Configuration and Environments

- **Environments:**
  - Local development.
  - Future hosted daemon environment.
- **Config loading approach:**
  - MVP uses built-in rule presets.
  - Future per-match configuration maps into explicit rule-profile structs.
  - File format is not chosen until external rule config is implemented.
- **Secret handling approach:**
  - No secrets in MVP.
  - Future daemon stores SSH host key path and AI provider credentials outside committed config.
- **Env var naming conventions:** future daemon variables use `DURAK_` prefix.

## 10. Observability and Operational Concerns

- **Logging strategy:**
  - Domain core does not log.
  - CLI prints user-facing messages, not operational logs.
  - Future daemon uses structured logs through an application-level logging port. `log/slog` is the default candidate because it is standard and handler-based, but adapters can use zap/zerolog handlers later if needed.
- **Request or correlation identifiers:**
  - Use match id and session id in daemon logs.
  - Use event sequence numbers for replay/debugging.
- **Metrics and tracing:**
  - None for MVP.
  - Future daemon can add session count, active match count, completed match count, and error counters before tracing.
- **Error reporting:**
  - CLI maps typed validation errors to clear user messages.
  - Future daemon logs operational errors with session/match context.
- **Operational health endpoints/checks:**
  - None for CLI.
  - Future daemon should support a simple local health command or endpoint if containerized.

## 11. Repository Layout

- **Top-level directories:**
  - `cmd/durak`: local CLI executable.
  - `cmd/durakd`: future SSH daemon executable.
  - `internal/domain`: framework-free game domain.
  - `internal/app`: session/application services and ports.
  - `internal/adapters/cli`: CLI parser/renderer/runner.
  - `internal/adapters/bot`: bot strategy implementations.
  - `internal/adapters/tui`: future Bubble Tea UI.
  - `internal/adapters/ssh`: future Wish integration.
  - `internal/adapters/storage`: future persistence adapters.
  - `docs`: planning and architecture documents.
- **Recommended initial layout for MVP:**
  - Start only with `cmd/durak`, `internal/domain`, `internal/app`, `internal/adapters/cli`, and `internal/adapters/bot`.
  - Add future adapter directories only when those milestones begin.

## 12. Deployment Shape

- **MVP deployables:** one local `durak` CLI binary.
- **Runtime dependencies:** none beyond the compiled binary.
- **Future deployables:**
  - `durak`: local CLI/TUI.
  - `durakd`: Wish SSH daemon.
- **Why this deployment shape is sufficient:** the first milestone validates game correctness locally. Server deployment becomes relevant only after the game loop and TUI/session model are stable.

## 13. Key Flows

### 13.1 Local CLI Match Flow

1. `cmd/durak` creates a session service with the built-in default rule preset and simple bot strategy.
2. Application layer starts a match.
3. Domain core shuffles/deals using injected randomness and emits setup events.
4. CLI renders a snapshot.
5. Human enters a command.
6. CLI parser converts input to a domain action request.
7. Application layer submits the action to the domain core.
8. Domain core validates, transitions state, and emits events.
9. If it is the bot's turn, application layer asks the strategy for an action and submits it through the same validation path.
10. Loop continues until the domain core reports match completion.

### 13.2 Future SSH Match Flow

1. `durakd` accepts an SSH session through Wish.
2. SSH adapter maps the connection to a player identity.
3. Session registry attaches the player to a table.
4. Wish starts a Bubble Tea program for the session.
5. TUI submits player actions to the application layer.
6. Match session serializes commands and mutates game state in order.
7. Persistence adapter records events and final summaries once enabled.

### 13.3 Future Match Completion Persistence Flow

1. Domain core emits match-end event.
2. Application layer builds match summary and requested rating/currency effects.
3. Persistence layer stores final events, summary, rating update, and currency ledger entries in one transaction.
4. Application layer returns the final visible result to active sessions.

## 14. Assumptions

- The first implementation does not need storage, networking, or background jobs.
- Rule configurability is represented in code first and externalized later.
- Active match state is in memory until persistence begins.
- The target architecture must support more than two seats even if the first implementation runs two seats only.
- Match event design should anticipate replay, statistics, and AI training.

## 15. Open Questions / Risks

- **Storage tool choice:** choose SQLite driver, query approach, and migration tool only when persistence work begins.
- **Event format:** JSON is the first candidate, but the durable event schema needs its own design before history/statistics work.
- **Event-store failure semantics:** current session code keeps events pending when append fails, but durable persistence must explicitly choose retry/blocking, rollback, or command/event transaction behavior before daemon mode relies on it.
- **Transfer rules:** default preset includes transfer behavior, but implementation can be phased after the base podkidnoy loop if needed.
- **SSH concurrency:** multiplayer daemon must serialize per-match mutations; do not let multiple sessions mutate match state directly.
- **Economy and rating:** rating/currency updates should be ledger-like and transactional when introduced.
