# Project Rules

Status: Approved.

## 1. Source of Truth

- **Mandatory:** Product behavior starts from `docs/2026-06-10-durak-prd.md`.
- **Mandatory:** Technology choices start from `docs/STACK.md`.
- **Mandatory:** Component boundaries, dependency direction, and repository layout start from `docs/ARCHITECTURE.md`.
- **Mandatory:** If documents conflict, resolve in this order: PRD for product intent, ARCHITECTURE for boundaries, STACK for technology choices, PROJECT_RULES for engineering policy.
- **Mandatory:** Any change that alters user-visible rules, module boundaries, storage strategy, deployment shape, or security posture must update the relevant doc in the same change.
- **Default:** Small implementation details do not need doc updates unless they change a decision future contributors would rely on.

## 2. Policy Mode

- **Default:** Use a balanced rule set.
- **Mandatory:** Security, secret handling, dependency direction, and game-rule test requirements are strict.
- **Mandatory:** Non-test source files must not exceed 700 LOC.
- **Review trigger:** File size around 300 LOC, function length, and complexity limits are prompts for review and refactoring before the hard LOC cap is reached.
- **Default:** Optimize for a small playable MVP without closing the path to TUI, SSH daemon, persistence, ratings, currency, DSL, and AI adapters.

## 3. General Engineering Rules

- **Mandatory:** Keep the domain core framework-free.
- **Mandatory:** Do not let CLI, TUI, SSH, storage, logging, or AI packages leak into `internal/domain`.
- **Mandatory:** Human and bot actions must pass through the same validation path.
- **Mandatory:** Invalid moves must be represented as typed validation errors that adapters can turn into user-facing messages.
- **Default:** Prefer simple data structures and explicit state transitions over clever generic abstractions.
- **Default:** Add abstractions only when they protect an architectural boundary or remove real duplication.
- **Review trigger:** If a change needs a large helper or shared package before two real call sites exist, justify why it is not premature.

## 4. Repository Conventions

- **Mandatory:** Initial core/CLI code belongs only in:
  - `cmd/durak`
  - `internal/domain`
  - `internal/app`
  - `internal/adapters/cli`
  - `internal/adapters/bot`
- **Mandatory:** Future adapter directories are added only when the milestone begins:
  - `internal/adapters/tui`
  - `internal/adapters/ssh`
  - `internal/adapters/storage`
- **Mandatory:** `cmd/*` packages contain wiring and executable entrypoints only.
- **Mandatory:** `internal/domain` owns game concepts and rule execution.
- **Mandatory:** `internal/app` owns orchestration, active sessions, ports, and adapter-facing use cases.
- **Mandatory:** `internal/adapters/*` packages translate external interaction styles into application calls.
- **Default:** Package names should be short, domain-specific, and not repeat parent directory names.
- **Review trigger:** A new top-level directory requires a documented reason.

## 5. Go Rules

- **Mandatory:** The repository must pin Go `1.26.1` in `go.mod` during bootstrap.
- **Mandatory:** Code may use language and standard-library features up to the pinned `go` directive, but not beyond it.
- **Default:** Prefer standard library features before adding dependencies.
- **Default:** Prefer modern Go helpers such as `errors.Is`, `errors.Join`, `slices`, `maps`, `cmp`, typed atomics, and context cancellation causes when the pinned Go version supports them.
- **Default:** Use `any` instead of `interface{}`.
- **Default:** Use `strings.Cut` / `CutPrefix` / `CutSuffix` instead of manual index slicing where it improves clarity.
- **Default:** Keep interfaces small and define them near the package that consumes them.
- **Mandatory:** Do not introduce package-level mutable state for active games.
- **Mandatory:** Randomness must be injectable for rule tests involving shuffle, redeal, trump selection, and random first-attacker fallback.
- **Review trigger:** A goroutine introduced before daemon/SSH work must justify its lifecycle, cancellation, and test strategy.

## 6. Design and Code-Writing Rules

- **Mandatory:** Business rules belong in domain code, not CLI/TUI/SSH adapters.
- **Mandatory:** Adapters may depend inward on `internal/app` and `internal/domain`; inward packages must not depend outward on adapters.
- **Mandatory:** Domain transitions must be deterministic for a given state, action, ruleset, and random source.
- **Mandatory:** Core state mutation should happen through explicit methods or functions that validate legal transitions.
- **Mandatory:** Do not expose mutable internal slices, maps, or structs from domain state. Return copies or read-only snapshots where needed.
- **Default:** Prefer named domain types over primitive strings/ints for suits, ranks, phases, seats, and action types.
- **Default:** Prefer explicit structs for actions and events over map-like payloads in core code.
- **Default:** Keep rendering models separate from domain models.
- **Mandatory:** Non-test source files must stay at or below 700 LOC.
- **Review trigger:** General source files around 300 LOC should be split or justified.
- **Review trigger:** Functions or methods over roughly 40-60 lines of executable logic should be split or justified.
- **Review trigger:** A function that mixes parsing, validation, mutation, and rendering should be split.

## 7. Rules by System Part

### Domain Core

- **Purpose:** rule-correct Durak state machine.
- **Mandatory:** No imports from Bubble Tea, Wish, storage drivers, logging libraries, AI clients, or CLI packages.
- **Mandatory:** Must expose legal actions or validation results rather than requiring adapters to infer legality.
- **Mandatory:** Must include tests for every implemented rule variant and edge case.
- **Default:** Events should be emitted close to the state transition that caused them.

### Application / Session Layer

- **Purpose:** orchestrate active matches and mediate between adapters, bots, events, and domain state.
- **Mandatory:** UI adapters cannot mutate domain state directly.
- **Mandatory:** Bot actions are submitted back through the same application path as human actions.
- **Default:** Keep session APIs adapter-neutral: no terminal concepts, no Bubble Tea messages, no SSH session types.
- **Review trigger:** Any application service that starts owning rule logic must be moved or justified.

### CLI Adapter

- **Purpose:** MVP user interaction through stdin/stdout.
- **Mandatory:** CLI parsing errors and domain validation errors must be distinct.
- **Mandatory:** CLI output must not be the source of truth for game state.
- **Default:** Prefer clear text commands over CLI frameworks until there are real subcommands to manage.
- **Default:** User-facing errors should say what action is legal now when practical.

### Bot Adapter

- **Purpose:** choose actions for bot-controlled seats.
- **Mandatory:** Bot strategies receive read-only decision context.
- **Mandatory:** Bot strategies must not mutate game state.
- **Mandatory:** Bot actions must be validated by domain/application logic.
- **Default:** The first bot should be deterministic enough to test, with randomness injected only when explicitly part of the strategy.

### Future TUI Adapter

- **Purpose:** richer terminal UI through Bubble Tea.
- **Mandatory:** Bubble Tea model owns presentation state only.
- **Mandatory:** TUI cannot duplicate rule validation.
- **Default:** Domain-to-view mapping should be isolated so the same game snapshot can serve CLI, TUI, and SSH.

### Future SSH Adapter

- **Purpose:** remote terminal play through Wish.
- **Mandatory:** Per-match state mutation must be serialized.
- **Mandatory:** SSH identity/session data must not leak into domain types.
- **Mandatory:** Future auth/session failures must be logged without exposing secrets or private keys.

### Future Persistence Adapter

- **Purpose:** match history, replay/training data, ratings, and currency ledger.
- **Mandatory:** Persistence consumes events and application commands; it does not own rule logic.
- **Mandatory:** Ledger-like changes for rating and currency must be transactional when introduced.
- **Default:** Select SQLite driver, query approach, and migration tool at the persistence milestone, not before.

## 8. Security and Secrets Rules

- **Mandatory:** Secrets never live in source code, committed config, docs, test fixtures, logs, or screenshots.
- **Mandatory:** Do not commit private keys, tokens, API keys, database credentials, or production-like secrets.
- **Mandatory:** Future `.env` files containing real values must be gitignored.
- **Mandatory:** Committed examples may use `.env.example` or sample config with fake values only.
- **Mandatory:** Validate untrusted boundary input: CLI commands, future SSH commands, future config files, future AI responses, and future persisted event imports.
- **Mandatory:** Logs must not include secrets, private keys, access tokens, full environment dumps, hidden player cards in public contexts, or raw AI prompts containing sensitive context.
- **Default:** Use least privilege for future daemon credentials and database accounts.
- **Review trigger:** Any new external service, AI provider, network listener, or persistence mechanism requires security review in the PR/changeset.

## 9. Configuration and Environment Rules

- **Mandatory:** MVP rule presets are code-defined until external config is intentionally added.
- **Mandatory:** Runtime config that varies by deploy belongs outside code once daemon mode exists.
- **Mandatory:** Future environment variables use the `DURAK_` prefix.
- **Mandatory:** Config and secrets are separate concepts; sample config may be committed, real secrets may not.
- **Default:** Keep config structs explicit and validate them at startup or match creation.
- **Review trigger:** Adding a config file format requires documenting schema, defaults, validation, and backward compatibility expectations.

## 10. Dependency Management Rules

- **Mandatory:** Use Go modules.
- **Mandatory:** Every new dependency must have a concrete reason tied to the current milestone.
- **Mandatory:** Domain core must use only the Go standard library unless explicitly approved.
- **Default:** Prefer small, maintained libraries with clear APIs over broad frameworks.
- **Default:** Avoid ORMs until persistence requirements prove they are useful.
- **Default:** For CLI MVP, avoid command frameworks unless real top-level subcommands exist.
- **Review trigger:** Adding a dependency to solve less than a few dozen lines of straightforward code requires justification.
- **Review trigger:** Adding a dependency with CGO, code generation, network access, or a large transitive graph requires explicit approval.

## 11. Code Quality and Hygiene Rules

- **Mandatory:** Run `gofmt` on all Go code.
- **Mandatory:** `golangci-lint run` is the mandatory project linter and must pass through `make lint`.
- **Mandatory:** `go test ./...` must pass before implementation work is considered done.
- **Mandatory:** `make check` must run formatting, linting, and tests before implementation work is considered done.
- **Default:** Keep `go vet ./...` available as a standalone diagnostic target even though `govet` is part of the lint profile.
- **Mandatory:** Non-test Go files over 700 LOC must be split before merge/commit.
- **Default:** Test files are exempt from the 700 LOC hard cap, but large tests should still be split when setup, fixtures, and assertions become hard to scan.
- **Mandatory:** Errors must preserve enough context for debugging while remaining usable by adapters.
- **Default:** Use error wrapping where callers need to classify errors with `errors.Is` or `errors.As`.
- **Mandatory:** Do not ignore returned errors unless the reason is documented and safe.
- **Review trigger:** Panics are allowed only for programmer errors or impossible states inside tests; normal gameplay errors must be returned.

## 12. Testing and Coverage Rules

- **Mandatory:** Domain rule logic must have unit tests.
- **Mandatory:** Every implemented rule from the default preset needs positive and negative tests where applicable.
- **Mandatory:** Bug fixes in domain/app logic require regression tests.
- **Mandatory:** Shuffle/deal/trump/first-attacker behavior must be testable with deterministic randomness.
- **Default:** Use table-driven tests for rule matrices and validation cases.
- **Default:** Use subtests with descriptive names for edge cases.
- **Default:** CLI tests can start with parser/renderer tests and smoke tests; full interactive tests can wait until the loop stabilizes.
- **Default:** No repo-wide coverage percentage is required for MVP.
- **Review trigger:** Low coverage in `internal/domain` is not acceptable even if repo-wide coverage looks high.
- **Review trigger:** Future persistence, rating, and currency code should have stricter tests than display-only code.

## 13. Git and Commit Rules

- **Default:** Keep commits atomic and reviewable.
- **Default:** Prefer Conventional Commit style once implementation begins: `feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `chore:`.
- **Mandatory:** Do not mix unrelated refactors with feature work unless required to make the feature safe.
- **Mandatory:** Do not commit generated binaries, local secrets, local databases, or transient logs.
- **Default:** Work does not need to end with a commit unless the user asks for one.

## 14. Documentation Update Rules

- **Mandatory:** Update PRD when product behavior, rule semantics, or scope changes.
- **Mandatory:** Update STACK when technology choices change.
- **Mandatory:** Update ARCHITECTURE when module boundaries, deployables, state ownership, or flows change.
- **Mandatory:** Update PROJECT_RULES when engineering policy changes.
- **Default:** Add short ADR-style notes only for decisions that are likely to be revisited or disputed.

## 15. Definition of Done for Implementation Tasks

- **Mandatory:** The change satisfies the requested behavior and preserves documented architecture boundaries.
- **Mandatory:** Relevant tests are added or updated.
- **Mandatory:** `gofmt` has been applied.
- **Mandatory:** `go test ./...` passes, or the failure is documented with the exact blocker.
- **Mandatory:** No new secrets, binaries, local DB files, or transient logs are committed.
- **Mandatory:** Relevant docs are updated when behavior, architecture, stack, or rules change.
- **Default:** Manual playtesting is required for user-facing CLI/TUI changes once the game loop exists.

## 16. Open Constraints or User Overrides

- **Approved:** Balanced policy mode.
- **Approved:** Non-test source files have a hard 700 LOC limit.
- **Approved:** Lower LOC and complexity thresholds remain review triggers, not hard limits.
- **Approved:** No repository-wide coverage threshold for MVP; domain coverage is risk-based and strict.
- **Approved:** Go target version is `1.26.1`.
- **Approved:** `internal/adapters/storage` may be used for the event-history milestone.
