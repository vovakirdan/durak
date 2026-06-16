# Heuristic Position Evaluation Task Breakdown

**Goal:** Build a seat-view-safe heuristic evaluator that scores Durak positions,
ranks legal actions, and can later power stronger offline bots.

**Approach:** Implement the evaluator as adapter-neutral application logic over
`app.DecisionContext`, not as domain rules and not as CLI rendering. Start with
score/result types, hidden-card modeling, static evaluation, and then action
scoring. Add a heuristic bot only after the evaluator has focused tests.

**Skills:** brainstorming, task-breakdown, writing-clearly-and-concisely,
typescript-best-practices not applicable, Go project rules from
`docs/PROJECT_RULES.md`.

**Tech Details:** Go 1.26.1, `internal/domain`, `internal/app`,
`internal/app/evaluation`, `internal/adapters/bot`, `cmd/durak`, `make check`,
`go test -race ./...`.

**Implementation Status:** Tasks 1-10 are complete for the first v1 pass. Arena
has an optional `-eval` move-quality summary for calibration runs, and SQLite
matches can be reviewed with `durak analyze`. The remaining work is richer
calibration and the later probabilistic shallow-search layer, not more
TUI/SSH/daemon integration.

---

## Architecture Notes

Create a new package under `internal/app/evaluation`. It may import
`github.com/vovakirdan/durak/internal/app` and `internal/domain`. The parent
`internal/app` package must not import `evaluation`; this avoids cycles and
keeps the domain core independent.

The evaluator must use seat-view data only. Do not clone `Session` or
`domain.Match` for v1 action scoring, because true match state can expose hidden
stock order after refill actions. V1 action scoring should combine static
position score with action-specific deltas. A later probabilistic search layer
can model expected hidden draws.

## Task 1: Core Evaluation Types

**Files:**

- Create: `internal/app/evaluation/score.go`
- Create: `internal/app/evaluation/score_test.go`
- Reference: `docs/2026-06-16-heuristic-position-evaluation-specs.md`

**Steps:**

1. Write tests for score clamping and move-quality thresholds.
2. Add `type Score int`.
3. Add constants for decisive bounds: `MinScore = -1000`, `MaxScore = 1000`.
4. Add `Clamp(score Score) Score`.
5. Add `MoveQuality` enum-like type with `best`, `good`, `inaccuracy`,
   `mistake`, `blunder`, and `brilliant`.
6. Add `QualityFromLoss(loss Score) MoveQuality` using the spec thresholds:
   `0..20`, `21..80`, `81..180`, `181..350`, and above `350`.
7. Run:

```sh
go test ./internal/app/evaluation
```

Expected: tests pass.

## Task 2: Evaluation Result Model

**Files:**

- Create: `internal/app/evaluation/result.go`
- Create: `internal/app/evaluation/result_test.go`

**Steps:**

1. Write tests that feature contributions sum to the final clamped score.
2. Add `FeatureContribution` with `Name string`, `Score Score`, and optional
   `Reason string`.
3. Add `PositionEvaluation` with `Seat domain.Seat`, `Score Score`,
   `Confidence int`, and `Features []FeatureContribution`.
4. Add a helper that builds a `PositionEvaluation` from feature contributions,
   clamps the score, and clamps confidence to `0..100`.
5. Keep feature names stable string constants. These names will later appear in
   training output and debugging.
6. Run:

```sh
go test ./internal/app/evaluation
```

Expected: tests pass.

## Task 3: Hidden-Card Model

**Files:**

- Create: `internal/app/evaluation/hidden_cards.go`
- Create: `internal/app/evaluation/hidden_cards_test.go`
- Reference: `internal/domain/card.go`
- Reference: `internal/domain/card_rules.go`
- Reference: `internal/app/view.go`

**Steps:**

1. Write a start-position test for a 36-card deck with a six-card hand and one
   visible trump indicator. Expected unknown pool size is `29`.
2. Write a late-game test where own hand, table, and discard leave a fully known
   opponent remainder.
3. Add `HiddenCards` with `Known []domain.Card`, `UnknownPool []domain.Card`,
   and per-card opponent probability helpers.
4. Build known cards from:
   - evaluated hand;
   - table attack and defense cards;
   - visible trump indicator;
   - optional public history cards when provided later.
5. For v1, accept `discard []domain.Card` as an explicit input to the builder.
   `DecisionContext` currently has only `DiscardCount`, so tests can use direct
   discard cards and production callers can pass none until public history is
   wired in.
6. Add `OpponentCardProbability(card domain.Card, opponentHandSize int) float64`
   as `opponentHandSize / len(UnknownPool)` when the card is unknown.
7. Run:

```sh
go test ./internal/app/evaluation
```

Expected: tests pass.

## Task 4: Static Position Evaluator

**Files:**

- Create: `internal/app/evaluation/evaluator.go`
- Create: `internal/app/evaluation/evaluator_test.go`
- Reference: `internal/app/view.go`
- Reference: `internal/domain/action.go`

**Steps:**

1. Write tests for clear sign behavior:
   - fewer own cards than a visible opponent is positive;
   - being defender with weak coverage is negative;
   - holding several high trumps is positive;
   - low confidence when unknown pool is large.
2. Add `Evaluator` with configurable `Weights`.
3. Add `DefaultWeights()` as constants, not learned values.
4. Add `Evaluate(decision app.DecisionContext, hidden HiddenCards) PositionEvaluation`.
5. Implement the first feature contributions:
   - material pressure;
   - trump strength;
   - defense coverage;
   - attack pressure;
   - role and phase risk;
   - endgame pressure;
   - uncertainty penalty.
6. Use `domain.CanBeat` for coverage checks.
7. Run:

```sh
go test ./internal/app/evaluation
```

Expected: tests pass.

## Task 5: Seat-View-Safe Action Scoring

**Files:**

- Create: `internal/app/evaluation/actions.go`
- Create: `internal/app/evaluation/actions_test.go`
- Reference: `internal/domain/action.go`

**Steps:**

1. Write tests that rank obvious legal actions:
   - defend with the cheapest sufficient card before taking;
   - avoid spending high trump when a non-trump defense works;
   - prefer shedding low non-trumps on attack when pressure is equal;
   - penalize `take` when a legal defense exists.
2. Add `ActionEvaluation` with `Action domain.Action`, `Score Score`,
   `Delta Score`, `Loss Score`, `Quality MoveQuality`, and optional
   `Features []FeatureContribution`.
3. Add `RankActions(decision app.DecisionContext, hidden HiddenCards) []ActionEvaluation`.
4. Score each legal action as static position score plus action-specific delta.
   Do not simulate hidden stock draws in v1.
5. Sort best action first. Ties should be stable and deterministic.
6. Run:

```sh
go test ./internal/app/evaluation
```

Expected: tests pass.

## Task 6: Heuristic Bot Controller

**Files:**

- Create: `internal/adapters/bot/heuristic.go`
- Create: `internal/adapters/bot/heuristic_test.go`
- Modify: `internal/adapters/bot/controller.go`
- Modify: `cmd/durak/controllers.go`
- Modify: `README.md`

**Steps:**

1. Write a bot test where `heuristic` chooses a better-ranked action than the
   old `simple` priority order in a controlled decision context.
2. Add `ControllerHeuristic = "heuristic"`.
3. Add `HeuristicController` that builds a v1 hidden-card model from the turn's
   seat view and hand, ranks legal actions, and returns the top action.
4. Wire `heuristic` into `bot.NewController`, `bot.ValidateControllerKind`,
   `cmd/durak.controllerNames`, and `cmd/durak.validatePlayerControllerKind`.
5. Update README controller lists and arena examples.
6. Run:

```sh
go test ./internal/adapters/bot ./cmd/durak
```

Expected: tests pass.

## Task 7: Arena Comparison Smoke

**Files:**

- Modify: `cmd/durak/sqlite_test.go` or add a focused arena test if needed.
- Optional: update `README.md`.

**Steps:**

1. Add a CLI-level smoke test for:

```sh
go run ./cmd/durak arena -matches 5 -seed 42 -p0 heuristic -p1 simple
```

2. Keep assertions modest: the command should finish, report both controller
   names, and stay under the action limit.
3. Do not assert that heuristic always wins. Early weights are not calibrated.
4. Run:

```sh
go test ./cmd/durak
```

Expected: tests pass.

## Task 8: Final Verification and Quality Check

**Files:**

- No new source files unless checks expose issues.

**Steps:**

1. Run formatting and full checks:

```sh
make check
make build
go test -race ./...
go test -cover ./...
```

2. Run a manual arena comparison:

```sh
bin/durak arena -matches 20 -seed 42 -max-actions 500 -p0 heuristic -p1 simple
```

3. Run Sentrux:

```text
sentrux scan /home/zov/projects/durak
sentrux health
sentrux check_rules
```

4. Accept small score movement only if architecture rules pass and modularity
   does not materially degrade.

## Task 9: Arena Evaluation Summary

**Files:**

- Create: `cmd/durak/arena_evaluation.go`
- Modify: `cmd/durak/arena.go`
- Modify: `cmd/durak/main_test.go`
- Modify: `README.md`

**Steps:**

1. Add optional `durak arena -eval`.
2. Wrap arena controllers with a diagnostic layer that sees the live
   `TurnContext`, ranks legal actions with the seat-view evaluator, and records
   the selected action's loss and quality.
3. Print aggregate move-quality counters and per-seat average loss.
4. Keep this in the CLI adapter layer; do not make `internal/app` import
   `internal/app/evaluation`.
5. Add a smoke test that only asserts the summary is printed.

## Task 10: Stored Match Move Analysis

**Files:**

- Create: `internal/app/evaluation/history.go`
- Create: `internal/app/evaluation/history_replay.go`
- Create: `internal/app/evaluation/history_test.go`
- Create: `cmd/durak/analyze.go`
- Modify: `cmd/durak/main.go`
- Modify: `cmd/durak/sqlite_test.go`
- Modify: `README.md`

**Steps:**

1. Read internal SQLite events for one match id.
2. Replay the match event by event and score each stored action before applying
   it.
3. Use only the acting seat's `DecisionContext`, public discard, public table,
   visible hand sizes, and legal actions.
4. Print aggregate move-quality counters plus the highest-loss moves.
5. Keep the CLI command as `durak analyze`; leave TUI/training rendering for a
   later surface.

## Deferred Work

- Public history into hidden-card model.
- Masked action preview for stock/refill actions.
- Probabilistic shallow search over plausible hidden hands.
- Persistent score tables and richer move-analysis reports.
- Training UI and oracle post-game analyzer.
