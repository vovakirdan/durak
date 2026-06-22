# Technology Stack

## 1. Context Summary

- **Product type:** terminal-first Durak game that starts as local offline CLI and grows into TUI, SSH-hosted multiplayer, persistent history, ratings, internal currency, bot strategy DSL, and AI-assisted bot decisions.
- **Primary user flow:** run the game in a Linux terminal, play a full match against a bot, then later connect to a hosted terminal session over SSH.
- **Key constraints:**
  - Keep the first implementation small and playable.
  - Keep game rules independent from CLI/TUI/SSH presentation.
  - Prefer a portable server-friendly artifact.
  - Make SSH gameplay a first-class target.
  - Do not introduce real-money mechanics.

## 2. System Shape

- **Relevant parts:**
  - Domain core for cards, rules, match state, validation, actions, and events.
  - Local CLI adapter for the first playable version.
  - Bot strategy adapter for algorithmic and future AI-driven decisions.
  - Future Bubble Tea TUI adapter.
  - Future Wish SSH daemon that hosts game sessions.
  - Future persistence layer for match history, ratings, currency, and replay/training data.
  - Future operational logging and observability layer for hosted daemon mode.
- **Why these parts exist:** the product needs a small local game first, but its target shape includes server-hosted terminal sessions and long-lived player/game data.

## 3. Interaction Surfaces

### Local CLI

- **Type:** CLI.
- **Framework/runtime:** Go standard library.
- **Delivery or rendering approach:** command loop with plain terminal output and text commands.
- **State/data access approach:** in-process game session; no persistence for the first core milestone.
- **Why this choice fits:** it keeps M1/M2 focused on rule correctness and playability instead of UI framework work.

### Terminal TUI

- **Type:** TUI.
- **Framework/runtime:** Go with Charm Bubble Tea, Bubbles, and Lip Gloss.
- **Delivery or rendering approach:** Bubble Tea model/update/view loop backed by the same domain core used by the CLI.
- **State/data access approach:** TUI owns presentation state only; match state remains in the game core/session layer.
- **Why this choice fits:** Bubble Tea gives a mature Go-native TUI path and aligns directly with the later Wish SSH server story.

### SSH Game Surface

- **Type:** SSH-hosted terminal application.
- **Framework/runtime:** Go with Charm Wish.
- **Delivery or rendering approach:** Wish hosts a Bubble Tea program per SSH session.
- **State/data access approach:** session service creates or resumes game sessions and talks to persistence once storage exists.
- **Why this choice fits:** SSH access is a target product feature, and Wish is built specifically for serving terminal applications over SSH.
- **Current transport decision:** choose Wish-hosted SSH before any HTTP,
  gRPC, or protobuf API. Revisit an API transport only when a non-terminal
  client, such as React, becomes a real target.

## 4. Execution and Service Components

### Game Core

- **Responsibility:** cards, deck, rule profiles, match setup, move validation, turn transitions, round resolution, event emission, and outcome detection.
- **Language/runtime:** Go.
- **Framework/approach:** framework-free package with explicit domain types and deterministic APIs.
- **Validation/contracts:** typed actions, validation results, rule-profile checks, table-driven tests, and deterministic RNG injection for tests.
- **Data access approach:** no direct storage access; emits events through an interface.
- **Why this choice fits:** game correctness should not depend on CLI, TUI, SSH, storage, or bot implementation details.

### Bot and AI Runtime

- **Responsibility:** choose legal actions for bot-controlled seats.
- **Language/runtime:** Go.
- **Framework/approach:** strategy interface with simple deterministic implementations first, plus a provider-neutral AI adapter for raw-command model responses.
- **AI provider library:** official `github.com/openai/openai-go/v3` SDK for OpenAI-compatible chat-completions endpoints.
- **Validation/contracts:** bot receives read-only decision context and returns an action that the game core validates.
- **Data access approach:** no storage access in the first implementation; future strategies may consume historical summaries through explicit ports.
- **Why this choice fits:** supports simple bots now and future DSL/AI strategies without giving bots mutable engine internals. The OpenAI-compatible API shape covers OpenAI, OpenRouter, LiteLLM, vLLM/Ollama-compatible servers, and similar providers without binding the game to one CLI helper transport.

### Future Daemon

- **Responsibility:** host SSH sessions, manage player identity, table/session lifecycle, persistence, and operational logs.
- **Language/runtime:** Go.
- **Framework/approach:** standard Go service process with Wish for SSH.
- **Validation/contracts:** daemon talks to the core through session/application services, not direct state mutation.
- **Data access approach:** repository interfaces backed by SQLite first. The exact Go data-access library and SQLite driver are selected at the persistence milestone, with portability and transaction clarity as the main criteria.
- **Why this choice fits:** a single Go service keeps deployment and runtime operations simple on a VPS.

## 5. Interfaces and Compatibility

- **Primary API style:** in-process interfaces for MVP; future daemon boundaries stay internal Go interfaces until a remote API is needed.
- **Remote transport order:** SSH/Wish first, API/protobuf later only with a
  concrete second frontend or process-boundary requirement.
- **Schema/documentation approach:** domain events and persisted records use versioned structs and stable serialization for replay/export compatibility. JSON is the first candidate because it is easy to inspect and export, not because it is the only acceptable format.
- **Auth/session strategy:** none for local CLI; future SSH mode starts with SSH key identity or server-managed player aliases.
- **Realtime approach:** local in-process event loop for CLI/TUI; one Bubble Tea program per SSH session under Wish for hosted play.
- **Compatibility notes across parts:**
  - CLI, TUI, and SSH must call the same application/session layer.
  - Game core must not import Bubble Tea, Wish, database drivers, or AI clients.
  - Bot strategies must submit normal player actions so validation remains centralized.
  - Persistence consumes domain events and snapshots; it does not own game rules.
- **Why these choices fit together:** the stack keeps Go across every first-class runtime while preserving enough boundaries to replace presentation or strategy layers later.

## 6. Data Layer

- **Primary database:** SQLite for the first indexed persistence/history milestone.
- **Go database access:** Bun for the runtime database framework and query layer, behind repository/application ports. Runtime storage code should use Bun models, transactions, and query builders rather than raw SQL strings.
- **Migrations:** Goose for versioned migrations with embedded migration files.
- **SQLite driver direction:** use Bun's SQLite shim and SQLite dialect for the first implementation, preserving the portable-binary direction. Re-evaluate CGO-based drivers only if SQLite extension support or performance becomes more important than build portability.
- **Cache or queue dependencies:** none for MVP or early daemon.
- **Search or analytics storage:** none initially; export match history/events for offline analysis before adding dedicated analytics storage.
- **Why this choice fits:** SQLite is enough for hosted private play, match history, ratings, and currency ledgers while keeping operations small. PostgreSQL remains an upgrade path, not an early dependency.

## 7. Async, Automation, or AI Components

- **Need for jobs/workflows/agents:** none in the first playable CLI.
- **Chosen approach:** explicit Go interfaces for future strategy engines and AI adapters.
- **Technology:** simple in-process algorithmic strategies, OpenAI-compatible HTTP adapter through `openai-go`, and a local subprocess escape hatch for debugging.
- **Why this choice fits:** it keeps the game loop deterministic and testable while leaving room for DSL and AI decision-making later.

## 8. Observability and Operations

- **Logging:** no logging dependency in the domain core. For daemon mode, use a structured logging port with a `log/slog`-compatible default unless a later benchmark or operational need justifies zap/zerolog.
- **Metrics:** none for local CLI; future daemon can expose Prometheus-compatible metrics if hosted use needs it.
- **Tracing:** none for MVP; add OpenTelemetry only if daemon complexity justifies it.
- **Error monitoring:** none for local CLI; daemon errors should start with structured logs and process supervision.
- **Why this choice fits:** early observability should help debug games and sessions without adding infrastructure before it is needed.

## 9. Dev Tooling

- **Package/dependency management:** Go modules.
- **Linting/formatting:** `gofmt` and `go vet` from the start. Add `golangci-lint` only after project rules define the lint profile, so early work is not blocked by noisy defaults.
- **Testing:** standard `go test`; table-driven unit tests for rules; deterministic RNG tests for dealing/trump/first-attacker policies; later integration tests for CLI and SSH session flows.
- **Migrations:** no migrations before persistence. Candidate tools for the first persistent version are Goose, golang-migrate, or a very small embedded migration runner if the schema remains tiny.
- **Containerization:** not required for local CLI; future daemon can ship as a static-ish Linux binary or small container image.
- **Local development workflow:** `go test ./...` as the baseline verification command; run the CLI binary locally for manual playtesting.
- **CI/CD baseline:** build, test, vet, and release Linux binaries; add container publish only when daemon mode begins.

## 10. Research Notes

- Bubble Tea uses a model/update/view architecture for terminal UIs, which maps cleanly to a UI adapter over a separate domain core: https://pkg.go.dev/github.com/charmbracelet/bubbletea
- Wish provides SSH app middleware for Bubble Tea and creates a Bubble Tea program per SSH session: https://github.com/charmbracelet/wish and https://pkg.go.dev/charm.land/wish/v2/bubbletea
- Ratatui is a strong Rust TUI library, but SSH-hosted TUI support is less direct for this product than Charm's Go stack: https://docs.rs/ratatui/latest/ratatui/
- Textual is a strong Python TUI framework with terminal and browser serving, but it weakens the single-portable-binary and Go/Wish SSH path: https://textual.textualize.io/
- `database/sql` is a standard Go abstraction with explicit connection-pool and transaction semantics, but using it directly everywhere can create manual scanning boilerplate: https://pkg.go.dev/database/sql
- Bun provides a Go ORM/query layer with SQLite support, transactions, migrations, and model-based query builders: https://bun.uptrace.dev/
- Goose provides versioned Go/SQL migrations and supports embedded migration files and SQLite: https://github.com/pressly/goose
- Bun's SQLite shim is the first SQLite driver direction for portability; `github.com/mattn/go-sqlite3` remains a mature CGO-based fallback if future SQLite extension support or performance requires it: https://bun.uptrace.dev/guide/drivers.html and https://github.com/mattn/go-sqlite3
- `log/slog` provides standard structured logging with pluggable handlers; zap and zerolog remain candidates if daemon performance or handler features require them: https://pkg.go.dev/log/slog, https://github.com/uber-go/zap, and https://github.com/rs/zerolog
- The official OpenAI Go SDK supports `option.WithBaseURL` and `option.WithAPIKey`, which is the smallest maintained path to OpenAI-compatible providers: https://github.com/openai/openai-go

## 11. Rejected Alternatives

- **Rust + Ratatui:** rejected as the primary stack because the product explicitly values SSH-hosted terminal play and simple deployment. Rust remains attractive for strict domain modeling, but would add more infrastructure work around SSH/session serving.
- **Python + Textual:** rejected as the primary stack because the target product benefits from a small portable Go server binary and the Charm SSH/TUI ecosystem. Textual remains a viable future prototype surface if the domain core is exposed through a stable service boundary.
- **Early PostgreSQL:** rejected because local CLI and private hosted play do not need a separate database server at the start.
- **Early web app:** rejected because terminal and SSH are first-class product surfaces; a browser UI would distract from the core game loop and terminal UX.
