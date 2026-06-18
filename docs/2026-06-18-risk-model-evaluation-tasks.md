# Risk Model Evaluation Task Breakdown

**Goal:** Replace the current action-delta evaluator with a risk-based position model that ranks moves by expected post-battle position.

**Approach:** Keep the existing public API (`Score`, `PositionEvaluation`, `ActionEvaluation`, `RankActions`) so arena, analyze, and bot wiring keep working. Implement the new model in small tested pieces: risk score scale, cover probability, battle risk, then action ranking. Do not add a new search engine or new controller family until the model beats the current baseline in mirrored arena checks.

**Skills:** @task-breakdown, @coding, @codebase-search

**Tech Details:** Go 1.26.1, stdlib only, `go test ./internal/app/evaluation`, `go test ./...`, `go run ./cmd/durak arena ...`

---

### Task 1: Freeze Current Contracts

**Files:**
- Modify: `internal/app/evaluation/score_test.go`
- Modify: `internal/app/evaluation/result_test.go`
- Modify: `internal/app/evaluation/actions_test.go`

**Step 1: Add contract tests**

Add tests that lock these public behaviors:

```go
func TestRiskScoreNeutralPointIsZero(t *testing.T) {
	got := evaluation.ScoreFromDurakProbability(0.5, 2)
	if got != 0 {
		t.Fatalf("score = %d, want neutral", got)
	}
}

func TestRankActionsKeepsStableActionEvaluationShape(t *testing.T) {
	decision := defenseDecision([]domain.Action{
		{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
		{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: card(domain.Seven, domain.Clubs), AttackIndex: 0},
	})
	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))
	if len(results) != 2 || results[0].Quality == "" {
		t.Fatalf("results = %+v, want ranked action evaluations", results)
	}
}
```

**Step 2: Run targeted tests**

Run: `go test ./internal/app/evaluation`

Expected: fail only for missing `ScoreFromDurakProbability`.

**Step 3: Add the smallest implementation**

Create the score conversion in `internal/app/evaluation/score.go`:

```go
func ScoreFromDurakProbability(probability float64, activePlayers int) Score {
	if activePlayers <= 1 {
		return MaxScore
	}
	if probability < 0 {
		probability = 0
	}
	if probability > 1 {
		probability = 1
	}
	neutral := 1 / float64(activePlayers)
	if probability <= neutral {
		return Clamp(Score(math.Round(1000 * (1 - float64(activePlayers)*probability))))
	}
	return Clamp(Score(math.Round(-1000 * (probability - neutral) / (1 - neutral))))
}
```

**Step 4: Verify**

Run: `go test ./internal/app/evaluation`

Expected: pass.

---

### Task 2: Add Risk Model Skeleton

**Files:**
- Create: `internal/app/evaluation/risk.go`
- Create: `internal/app/evaluation/risk_test.go`
- Modify: `internal/app/evaluation/evaluator.go`

**Step 1: Write failing tests**

Cover the Excel-backed invariants before tuning weights:

```go
func TestRiskModelFewerCardsIsBetterWhenStockEmpty(t *testing.T) {
	low := riskDecision(0, []int{2, 6}, 0)
	high := riskDecision(0, []int{6, 2}, 0)
	model := evaluation.DefaultRiskModel()
	if model.Evaluate(&low, evaluation.BuildHiddenCards(&low, nil)).Score <=
		model.Evaluate(&high, evaluation.BuildHiddenCards(&high, nil)).Score {
		t.Fatalf("fewer cards with empty stock should score higher")
	}
}

func TestRiskModelFullStockDoesNotOverrewardShortHand(t *testing.T) {
	low := riskDecision(0, []int{2, 6}, 20)
	normal := riskDecision(0, []int{6, 6}, 20)
	model := evaluation.DefaultRiskModel()
	if model.Evaluate(&low, evaluation.BuildHiddenCards(&low, nil)).Score-model.Evaluate(&normal, evaluation.BuildHiddenCards(&normal, nil)).Score > 250 {
		t.Fatalf("short hand with full stock should not be treated as nearly won")
	}
}
```

**Step 2: Implement `RiskModel`**

Add:

```go
type RiskWeights struct {
	Beta              float64
	StockFinalityBase float64
	HandBurden        float64
	Threat            float64
	Outlet            float64
	DefenseStability  float64
	Initiative        float64
}

type RiskModel struct {
	Weights RiskWeights
}

func DefaultRiskModel() RiskModel
func (m RiskModel) Evaluate(decision *app.DecisionContext, hidden HiddenCards) PositionEvaluation
```

Use one unit as "one bad card". Keep weights in this file so Excel constants can be pasted in one place.

**Step 3: Wire `Evaluate`**

Change `Evaluate(decision, hidden)` to call `DefaultRiskModel().Evaluate(...)`.

**Step 4: Verify**

Run: `go test ./internal/app/evaluation`

Expected: pass.

---

### Task 3: Cover Probability And Matching

**Files:**
- Create: `internal/app/evaluation/cover.go`
- Create: `internal/app/evaluation/cover_test.go`

**Step 1: Add tests**

```go
func TestSinglePendingCoverProbabilityUsesKnownHeld(t *testing.T) {
	attack := card(domain.Ace, domain.Clubs)
	known := card(domain.Six, domain.Hearts)
	hidden := evaluation.HiddenCards{
		Seat: domain.Seat(0),
		KnownHeld: &[][]domain.Card{{}, {known}},
	}
	got := evaluation.CoverProbability([]domain.Card{attack}, domain.Seat(1), 1, domain.Hearts, hidden)
	if got != 1 {
		t.Fatalf("probability = %v, want known cover", got)
	}
}

func TestCanCoverAllRequiresDistinctDefenseCards(t *testing.T) {
	pending := []domain.Card{card(domain.Six, domain.Clubs), card(domain.Seven, domain.Clubs)}
	hand := []domain.Card{card(domain.Ace, domain.Clubs)}
	if evaluation.CanCoverAll(pending, hand, domain.Hearts) {
		t.Fatalf("one card must not cover two attacks")
	}
}
```

**Step 2: Implement exact helpers**

Add:

```go
func CoverProbability(pending []domain.Card, defender domain.Seat, defenderHandSize int, trump domain.Suit, hidden HiddenCards) float64
func CanCoverAll(pending []domain.Card, hand []domain.Card, trump domain.Suit) bool
```

Use closed-form hypergeometric for one pending card. Use small DFS matching for multiple cards; no dependency.

**Step 3: Verify**

Run: `go test ./internal/app/evaluation -run 'Cover|CanCover'`

Expected: pass.

---

### Task 4: Battle Risk For Defender

**Files:**
- Create: `internal/app/evaluation/battle.go`
- Create: `internal/app/evaluation/battle_test.go`
- Modify: `internal/app/evaluation/risk.go`

**Step 1: Add scenario tests**

Test the cases the old model cannot express:

```go
func TestBattleRiskCanPreferTakingOverBurningLastTrump(t *testing.T)
func TestBattleRiskPrefersCheapDefenseOverTakingHeavyTable(t *testing.T)
func TestDefenseCardRiskIncludesOpenedRankPressure(t *testing.T)
```

**Step 2: Implement battle terms**

Add:

```go
type BattleRisk struct {
	TakeNow         float64
	ContinueDefense float64
	Transfer       float64
	Best           float64
}

func EvaluateBattleRisk(decision *app.DecisionContext, hidden HiddenCards) BattleRisk
```

Use:
- table incoming burden;
- spend cost for defense card;
- rank opening pressure from `hidden.UnknownPool` and `KnownHeld`;
- initiative loss for taking.

**Step 3: Feed risk model**

For current defender, add `BattleRisk.Best` into `RiskIndex`.

**Step 4: Verify**

Run: `go test ./internal/app/evaluation -run 'Battle|RiskModel'`

Expected: pass.

---

### Task 5: Rewrite Action Ranking

**Files:**
- Modify: `internal/app/evaluation/actions.go`
- Modify: `internal/app/evaluation/actions_test.go`
- Modify: `internal/adapters/bot/heuristic.go`

**Step 1: Add ranking tests**

```go
func TestRankActionsUsesPostBattleRiskInsteadOfActionDelta(t *testing.T)
func TestRankActionsCanChooseTakeWhenDefenseBurnsCriticalTrump(t *testing.T)
func TestRankActionsAvoidsThrowInThatOnlyHelpsStrongDefender(t *testing.T)
```

**Step 2: Replace `base + delta`**

Change `RankActions` to:

```go
for _, action := range decision.LegalActions {
	score := ScoreAction(decision, hidden, action)
	...
}
```

`ScoreAction` should project only the minimal public effect needed for the current action and then score with `RiskModel`. Keep the code local and deterministic.

**Step 3: Keep output compatibility**

Still fill `ActionEvaluation.Score`, `Delta`, `Loss`, `Quality`, and useful `Features`.

**Step 4: Verify**

Run:

```bash
go test ./internal/app/evaluation ./internal/adapters/bot
```

Expected: pass.

---

### Task 6: Collapse Experimental Controller Surface

**Files:**
- Modify: `internal/adapters/bot/controller.go`
- Modify: `internal/adapters/bot/heuristic.go`
- Modify: `cmd/durak/controllers.go`
- Modify: `internal/adapters/bot/controller_test.go`

**Step 1: Remove unused P1 variants**

Delete controller kinds that only exist for projection experiments:
- `heuristic-p1-norole`
- `heuristic-p1-transfer-delta`
- `heuristic-p1-take-quality`
- `heuristic-p1-take-quality-transfer-delta`
- `heuristic-p1-take-dump`
- `heuristic-p1-transfer-response`

Keep `heuristic` as the production risk model. Do not keep a projection/P1
controller as an arena comparison; the old model is not a baseline for this
rewrite.

**Step 2: Verify CLI names**

Run:

```bash
go test ./internal/adapters/bot ./cmd/durak
```

Expected: pass.

---

### Task 7: Arena Validation And Logging

**Files:**
- Modify: `cmd/durak/arena_evaluation.go`
- Modify: `cmd/durak/arena.go`
- Modify: `docs/EVALUATION_MODEL.md`

**Step 1: Extend eval log only where useful**

Add fields:

```text
risk_score
battle_take
battle_defend
battle_transfer
cover_probability
```

Skip broad per-feature dumps until needed.

**Step 2: Run mirrored smoke**

Run:

```bash
go run ./cmd/durak arena -matches 100 -seed 202 -max-actions 500 -p0 heuristic -p1 simple -eval
go run ./cmd/durak arena -matches 100 -seed 202 -max-actions 500 -p0 simple -p1 heuristic -eval
```

Expected: no illegal actions, no panic, and action distribution includes meaningful `take` and `throw_in` decisions.

**Step 3: Run full check**

Run: `go test ./...`

Expected: pass.

---

### Task 8: Document The New Current Model

**Files:**
- Modify: `docs/EVALUATION_MODEL.md`
- Keep: `model_conversation/dialog.md`
- Keep: `temp_deep_research.md`

**Step 1: Rewrite docs after code is true**

Document only implemented behavior:
- risk index;
- score normalization;
- public memory and hidden cards;
- cover probability;
- battle risk;
- action ranking.

**Step 2: Add known gaps**

Call out:
- no package initial attack yet because domain legal actions are one-card attacks;
- no full belief sampling yet;
- no god-mode evaluator yet;
- no learned weights.

**Step 3: Verify docs and tests**

Run: `go test ./...`

Expected: pass.
