# Architecture

## 1. Context

- **Product shape:** terminal-first Durak game. The MVP is local CLI play against a bot. The target system adds Bubble Tea TUI, Wish SSH hosting, multiplayer, persistent match history, ratings, internal currency, strategy DSL, and AI-backed bot decisions.
- **Relevant truth sources:**
  - `docs/2026-06-10-durak-prd.md`
  - `docs/STACK.md`
  - `docs/2026-06-15-match-config-specs.md`
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
- **Application/session layer:** coordinates matches and optional in-memory series, accepts player decisions, runs headless games, invokes player controllers, owns active in-memory state, and exposes snapshots to adapters.
- **Match configuration model:** app-level value objects for per-match rules, seats, and series behavior. They validate built-in or future persisted config before mapping to domain rule profiles.
- **CLI adapter:** parses terminal commands, renders text output, and calls the application layer.
- **Text command adapter:** parses shared terminal-style player commands for CLI humans and raw-command AI testers.
- **Bot adapter:** implements strategy interfaces using read-only decision contexts.
- **AI adapter:** defines provider-neutral AI client contracts and adapts raw AI command responses into player decisions.
- **Command wiring:** maps CLI flags to app-facing rule profiles and player controllers without embedding gameplay decisions in executable entrypoints.
- **Future TUI adapter:** Bubble Tea presentation and input layer over the same application/session layer.
- **Future SSH adapter:** Wish server/session bridge that hosts TUI sessions remotely.
- **Future persistence adapter:** event/snapshot storage, match history, ratings, and currency ledgers.

## 4. Interaction Surfaces

### Local CLI

- **Type:** CLI.
- **Primary responsibilities:** start a game, show visible state, parse commands, print validation errors, and drive one local human seat against controller-driven seats.
- **Internal structure:** command parser, renderer, and loop runner.
- **Key dependencies:** application/session layer and text command adapter only.
- **Current boundary:** supports one local human seat with 2..6 total seats; true multi-human input belongs to the future TUI/SSH table surfaces.

### Raw AI Command Tester

- **Type:** headless player-controller adapter.
- **Primary responsibilities:** build a seat-scoped prompt, ask an AI-like client for a text command, parse that command through the shared text command adapter, and return a normal player decision.
- **Internal structure:** prompt builder, provider-neutral AI client port, raw command controller, OpenAI-compatible HTTP client, optional subprocess client, and trace records for raw response and parse result.
- **Key dependencies:** application/session layer and text command adapter. It must not call CLI render loops or mutate sessions directly.

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
  - throw-in window state: eligible seats, lead-first opening, pass tracking, attack participants, and attack-card limits.
  - action types and validation.
  - event types and visibility metadata.
  - deterministic shuffle/deal helpers using injected randomness.
- **Key dependencies:** Go standard library only.
- **Why this boundary exists:** it keeps the most important logic testable and reusable across CLI, TUI, SSH, bots, and future storage.

### Application/Session Layer

- **Responsibility:** orchestrate active matches and expose a stable API for adapters.
- **Internal module structure:**
  - session service.
  - series/table orchestration for consecutive matches.
  - match config value objects for seats, rule presets, and future live-loaded rule records.
  - headless series runner for bot-vs-bot, scripted, and future remote-controller games.
  - player controller ports that adapt bots, humans, scripts, or future external processes.
  - player/seat registry for the active match.
  - command handling: submit action, ask bot, advance state.
  - snapshot/query methods for rendering.
  - event store port.
- **Key dependencies:** domain core, bot strategy interfaces, optional event store.
- **Why this boundary exists:** UI adapters should not mutate match state directly.

### Bot Strategy Layer

- **Responsibility:** choose actions for bot-controlled seats.
- **Internal module structure:**
  - controller registry for built-in bot/controller kinds.
  - strategy interface.
  - read-only decision context.
  - simple deterministic strategy for MVP.
  - random legal-action controller for smoke testing.
  - future DSL and AI strategy adapters.
- **Key dependencies:** domain action/value types only.
- **Why this boundary exists:** bot decisions must be replaceable and validated through the same path as human actions.

### AI Player Adapter Layer

- **Responsibility:** adapt external or mock AI clients into `PlayerController` implementations.
- **Internal module structure:**
  - provider-neutral `Client` interface.
  - raw-command prompt/response models.
  - raw command controller for parser and validation stress tests.
  - OpenAI-compatible chat-completions client using the official OpenAI Go SDK.
  - subprocess raw-command client for local wrapper scripts or external LLM CLIs.
  - private JSONL trace sink for opt-in AI prompt/response diagnostics.
  - future structured AI controller that selects action IDs or decisions directly.
- **Key dependencies:** application/session types, domain value types, text command parsing, the official OpenAI Go SDK for compatible HTTP endpoints, and standard-library process execution for local wrappers. Provider SDKs must stay behind this adapter when introduced.
- **Why this boundary exists:** AI players should be replaceable and observable without coupling the game core or CLI/TUI loops to model providers.

### Persistence Layer

- **Responsibility:** durable match history, replay/training data, player profiles, rating state, and currency ledger.
- **Internal module structure:**
  - JSONL event store for the first local public-history milestone.
  - SQLite event store for future daemon/history mode.
  - snapshot store if replay from full event history becomes expensive.
  - repositories for players, ratings, and currency.
  - migration runner.
- **Key dependencies:** standard library for JSONL; storage library chosen later for SQLite.
- **Why this boundary exists:** persistence should consume stable events and application commands without owning game rules.

## 6. Interfaces and Boundaries

- **Primary contracts/interfaces:**
  - `SessionService`: starts matches, accepts actions, advances bot turns, returns render snapshots.
  - `Series`: links optional consecutive matches at one table through stable seat order, `MatchConfig`, and completed match results.
  - `MatchConfig`: app-level match creation config that validates rule, seat, and series options before a match starts; `domain.RuleProfile` is derived from it.
  - `PlayerController`: receives a read-only turn context and returns a player decision such as a legal action or concession.
  - `ai.Client`: receives a provider-neutral turn prompt and returns a raw or structured AI response.
  - `SeriesRunner`: executes a series without CLI/TUI ownership and records a compact decision trace.
  - `Strategy`: receives decision context and returns a proposed action.
  - `EventStore`: appends structured domain/application events for one match stream.
  - `RandomSource`: enables deterministic tests for shuffle and first-attacker fallback.
  - Future `PlayerStore`, `MatchStore`, `RatingStore`, and `CurrencyLedger`.
- **How parts communicate:**
  - CLI/TUI/SSH call application services.
  - Application services call domain core.
  - Player controllers return decisions; strategy-based controllers adapt bot strategies into ordinary player decisions.
  - Raw AI command controllers parse model text through the same text command adapter used by CLI, then return ordinary player decisions.
  - Bot strategies return proposed domain actions and remain replaceable by DSL, heuristic, AI, or external-process engines later.
  - Persistence consumes application event streams and stores snapshots/records later.
- **API/event/schema strategy:**
  - In-process Go interfaces for MVP.
  - Application events include match id and sequence number around structured domain event payloads.
  - Stable serialized events use a JSON envelope: `schema_version`, `match_id`, `sequence`, `kind`, `visibility`, and `payload`.
  - `match_started.payload.config` stores the app-level match config identity: config schema version, rule preset, derived rule profile, and a stable config hash. It intentionally does not store the full config snapshot yet.
  - The canonical match source of truth is an internal event stream. Public events are a safe projection/export of that stream, not the full replay source.
  - Current CLI JSONL output writes only `visibility=public` events. It is suitable for visible history and debugging, but it is not sufficient for exact resume or AI training data.
  - Internal events use `visibility=internal` and may include hidden state such as initial hands and stock order. Hidden state must never be mixed into public replay data.
  - Durable adapters may add record metadata such as insertion timestamp outside the envelope when a database store is introduced.
  - JSONL stores one event envelope per line and is intended for local debugging, replay smoke tests, and export.
  - SQLite remains the target store for indexed history, projections, ratings, currency, and daemon concurrency.
- **Compatibility notes across parts:**
  - Domain core imports no adapter packages.
  - Adapters may depend inward on application/domain packages.
  - Bot and human actions share the same validation path.
  - Optional throw-ins are modeled as a free-form domain window, not as UI turn order. Any eligible seat may submit a legal throw-in; adapters such as arena may poll seats deterministically, while future daemon/TUI sessions can accept concurrent player commands and serialize accepted actions per match.
  - Future SSH concurrency must serialize mutations per match.

## 7. Data and State Architecture

- **Core entities and ownership:**
  - Domain core owns card, deck, rule, match, round, table, action, and event semantics.
  - Application config owns external-facing rule choices and maps them into domain profiles; it does not execute rules.
  - Throw-in policy is represented by typed rule fields for player scope, timing, opening, close behavior, and contextual attack limits. The default preset is `all except defender`, `any eligible`, `lead first`, and `all eligible passed`, with no defender-hand attack cap.
  - Application/session layer owns active in-memory match sessions and in-memory series/table state.
  - Future persistence owns durable records, not live rule execution.
- **Event stream roles:**
  - Public events are safe for visible history, public replay, and local JSONL export.
  - Internal events are the canonical per-match stream for exact replay/resume. The initial internal deal records hidden hands and stock order; later accepted actions can be replayed from that state.
  - Config identity is attached to the initial `match_started` event in both public and internal streams. Future durable stores can use this identity to resolve or verify the full rule snapshot before replay, statistics, or model-training export.
  - `pass_throw_in` is a first-class action event because declining an optional throw-in affects whether a round can close and must be replayable.
  - Future internal events may include additional hidden state such as explicit draw cards, seeds, or decision context when needed for faster reconstruction or model training.
  - Snapshots are derived checkpoints for faster resume/replay; they are not the source of truth.
  - Projections/read models are derived tables for statistics, scoring, ratings, and global analytics; they must be rebuildable from event streams.
  - Match-history projections must model participants as `seats[]` plus seat-based fields (`winner`, `loser`, `first_attacker`, `defender`, `hand_sizes[]`) rather than `player1`/`player2` columns.
- **Match and series boundaries:**
  - A match is self-contained: rules, seats, initial deal, actions, and outcome must be enough to replay/analyze that match without reading a previous match.
  - A series/table session links optional consecutive matches through `series_id`, stable `seat_order`, match ids, completed match results, and previous loser metadata.
  - The "next match starts before the previous loser" rule is an input to new-match creation from series/table state, not a hidden dependency inside an old match stream.
  - The app layer applies the consecutive-match first-attacker override after deal setup and before `NewMatch`; the resulting initial deal is still recorded as the canonical internal event for replay.
  - Multiplayer adapters must expose per-seat views from internal state; public streams are not player-private views.
- **Primary storage responsibilities:**
  - MVP before event-history milestone: no durable storage.
  - First event-history milestone: append public match events to JSONL.
  - First persistence milestone: append match events to SQLite and store completed match summaries derived from the N-seat-safe history projection.
  - Later milestones: player profile, rating records, and currency ledger entries.
- **Transaction boundaries:**
  - JSONL: single-process append with batch validation before write; no cross-record transaction guarantees.
  - Dual public/internal writes are not treated as an atomic durable transaction in the local JSONL milestone. Daemon persistence must write the canonical internal event and its public projection transactionally or rebuild the public projection from internal events.
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
  - MVP uses built-in Go rule presets exposed by CLI flags, starting with `-rules default`.
  - The first config contract is an in-process `MatchConfig`/`RuleConfig` value object, not a file format.
  - `MatchConfigIdentity` is derived from a validated config and is recorded at match start. It gives history and future DB rows a compact reference before external config snapshots exist.
  - Bot/controller selection is also flag-driven in CLI and arena, starting with `simple` and `random`.
  - Future per-match configuration sources may be files, database records, or daemon control-plane requests.
  - Runners receive immutable config values at match creation; live config updates affect new matches unless an explicit migration/admin flow says otherwise.
  - File format is not chosen until external file config is intentionally implemented.
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
  - `internal/app`: session/application services, match config value objects, and ports.
  - `internal/adapters/cli`: CLI renderer and interactive loop runner.
  - `internal/adapters/textcmd`: shared terminal-style player command parsing.
  - `internal/adapters/bot`: bot strategy implementations.
  - `internal/adapters/ai`: provider-neutral AI player adapters.
  - `internal/adapters/tui`: future Bubble Tea UI.
  - `internal/adapters/ssh`: future Wish integration.
  - `internal/adapters/storage`: future persistence adapters.
  - `docs`: planning and architecture documents.
- **Recommended current layout for MVP:**
  - Keep active implementation in `cmd/durak`, `internal/domain`, `internal/app`, `internal/adapters/cli`, `internal/adapters/textcmd`, `internal/adapters/bot`, `internal/adapters/ai`, and `internal/adapters/storage`.
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

1. `cmd/durak` maps CLI flags to a built-in rule preset and bot controller.
2. Application layer starts a match.
3. Domain core shuffles/deals using injected randomness and emits setup events.
4. CLI renders a snapshot.
5. Human enters a command.
6. The text command adapter converts input to a command or domain action request.
7. Application layer submits the action to the domain core.
8. Domain core validates, transitions state, and emits events.
9. If it is the bot's turn, CLI asks the configured player controller for a decision and submits it through the same application validation path.
10. Loop continues until the domain core reports match completion.

### 13.2 Local Series Match Flow

1. Adapter or future table service creates an app-level `Series` with `series_id`, stable `seat_order`, and `MatchConfig`.
2. Series starts a match by dealing through the domain setup helper.
3. If `SeriesConfig.Consecutive` is enabled and the previous completed match had a loser, series sets the first attacker to the seat before that loser before creating the domain match.
4. Session emits the canonical internal deal event with the final first attacker value, so the match can be replayed without reading prior matches.
5. When the match completes, series records winner/loser/draw and updates previous-loser state for the next match.
6. Draw-like completion clears previous-loser state, so the next match falls back to normal setup rules.

### 13.3 Headless Runner Flow

1. Tests, bot labs, or future automation configure a `Series`, seat controllers, deal options, and a max-actions guard.
2. Runner starts each match through `Series`, preserving the same setup and event path used by adapters.
3. Runner asks the active seat's `PlayerController` for one decision from a copied read-only turn context.
4. Runner validates returned actions against legal actions or applies concession through the same session path.
5. Runner stops with a typed error if a controller is missing, returns an illegal decision, or exceeds the action limit.
6. Runner returns completed match summaries plus a compact decision trace for smoke tests and future analysis.

### 13.4 Raw AI Command Flow

1. Runner or a future table service invokes a raw AI `PlayerController` for the active AI seat.
2. The controller builds a prompt from the copied `TurnContext`, including visible state, private hand, and legal action command hints.
3. The configured `ai.Client` returns one raw text command.
4. The controller parses the command through `internal/adapters/textcmd`.
5. Parser errors, non-player commands, or illegal choices are returned as controller errors and recorded in controller trace for diagnostics.
6. Valid action or concession commands become `PlayerDecision` values and continue through `Session.ApplyPlayerDecision`.

When `-ai-trace-log` is configured, the raw AI controller appends a private
JSONL record for each attempt. The record includes schema version, non-secret
provider metadata, timing, prompt payload, raw command, token usage when the
provider reports it, parsed command kind, player decision when valid, and error
text when invalid. This trace is diagnostic/training input for AI work and is
separate from the public match event log.

For `ai-openai`, the client serializes the provider-neutral prompt into a chat
completion request and sends it to the configured OpenAI-compatible base URL
with the configured model and API key. The response text is still parsed through
the same raw command controller.

For `ai-raw-exec`, the client serializes a provider-neutral prompt as JSON to
the external process stdin and treats the first non-empty stdout line as the raw
command. This path is a debug/local-wrapper escape hatch, not the primary AI
provider integration.

### 13.5 Future SSH Match Flow

1. `durakd` accepts an SSH session through Wish.
2. SSH adapter maps the connection to a player identity.
3. Session registry attaches the player to a table.
4. Wish starts a Bubble Tea program for the session.
5. TUI submits player actions to the application layer.
6. Match session serializes commands and mutates game state in order.
7. Persistence adapter records events and final summaries once enabled.

### 13.6 Future Match Completion Persistence Flow

1. Domain core emits match-end event.
2. Application layer builds match summary and requested rating/currency effects.
3. Persistence layer stores final events, summary, rating update, and currency ledger entries in one transaction.
4. Application layer returns the final visible result to active sessions.

## 14. Assumptions

- The local CLI can run without storage; optional JSONL event export is a local history/debugging adapter, not daemon persistence.
- Rule configurability is represented in code first and externalized later.
- Active match state is in memory until persistence begins.
- The domain and headless runner support 2..6 canonical seats; the interactive CLI remains a two-seat surface until the TUI/table UX is ready.
- Match event design should anticipate replay, statistics, and AI training.
- Raw AI command testing is a QA/stability tool first; provider-backed structured AI players are a later integration layer.

## 15. Open Questions / Risks

- **Storage tool choice:** JSONL is accepted only as the first local event-history adapter; choose SQLite driver, query approach, and migration tool when indexed persistence begins.
- **Event schema evolution:** JSON envelope v1 now exists for public and internal streams; future persistence work must define migration/version handling for SQLite-backed history.
- **Event-store failure semantics:** current session code keeps events pending when append fails, but durable persistence must explicitly choose retry/blocking, rollback, or command/event transaction behavior before daemon mode relies on it.
- **Transfer rules:** default preset includes transfer behavior, but implementation can be phased after the base podkidnoy loop if needed.
- **Series durability:** current series/table state is in memory only; daemon mode must persist series metadata and completed match summaries before supporting hosted consecutive tables.
- **SSH concurrency:** multiplayer daemon must serialize per-match mutations; do not let multiple sessions mutate match state directly.
- **Economy and rating:** rating/currency updates should be ledger-like and transactional when introduced.
- **AI provider execution:** real model providers need timeout, retry, redaction, and secret-handling rules before they are enabled outside mock/local tests.
