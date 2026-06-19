package evaluation_test

import (
	"math"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRiskModelFewerCardsIsBetterWhenStockEmpty(t *testing.T) {
	low := riskDecision([]int{2, 6}, 0)
	high := riskDecision([]int{6, 2}, 0)
	model := evaluation.DefaultRiskModel()

	lowScore := model.Evaluate(&low, evaluation.BuildHiddenCards(&low, nil)).Score
	highScore := model.Evaluate(&high, evaluation.BuildHiddenCards(&high, nil)).Score

	if lowScore <= highScore {
		t.Fatalf("low score = %d, high score = %d; fewer cards with empty stock should score higher",
			lowScore, highScore)
	}
}

func TestRiskModelFullStockDoesNotOverrewardShortHand(t *testing.T) {
	low := riskDecision([]int{2, 6}, 20)
	normal := riskDecision([]int{6, 6}, 20)
	model := evaluation.DefaultRiskModel()

	lowScore := model.Evaluate(&low, evaluation.BuildHiddenCards(&low, nil)).Score
	normalScore := model.Evaluate(&normal, evaluation.BuildHiddenCards(&normal, nil)).Score

	if lowScore-normalScore > 250 {
		t.Fatalf("low score = %d, normal score = %d; short hand with full stock is overrewarded",
			lowScore, normalScore)
	}
}

func TestRiskModelHandBurdenUsesExcelPhaseWeight(t *testing.T) {
	decision := riskDecision([]int{8, 6}, 22)
	components := evaluation.DefaultRiskModel().Components(
		&decision,
		evaluation.BuildHiddenCards(&decision, nil),
	)

	if !almostEqual(components[0].HandBurden, 1.7179166667) {
		t.Fatalf("HandBurden = %.6f, want Excel phase-weighted stickiness",
			components[0].HandBurden)
	}
}

func TestEvaluateUsesRiskModel(t *testing.T) {
	decision := riskDecision([]int{2, 6}, 0)

	result := evaluation.Evaluate(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if result.Score <= 0 {
		t.Fatalf("Score = %d, want positive risk score", result.Score)
	}
	if riskFeatureScore(result) != result.Score {
		t.Fatalf("risk feature = %d, want score %d",
			riskFeatureScore(result), result.Score)
	}
}

func TestRiskModelMatchesSpreadsheetBattleFixture(t *testing.T) {
	p1 := []domain.Card{
		card(domain.Six, domain.Hearts),
		card(domain.Seven, domain.Hearts),
		card(domain.Ten, domain.Clubs),
		card(domain.Queen, domain.Spades),
		card(domain.Ace, domain.Spades),
	}
	p2 := []domain.Card{
		card(domain.Six, domain.Spades),
		card(domain.Nine, domain.Clubs),
		card(domain.Ten, domain.Hearts),
		card(domain.Jack, domain.Diamonds),
		card(domain.King, domain.Clubs),
	}
	discard := []domain.Card{
		card(domain.Six, domain.Diamonds),
		card(domain.Seven, domain.Diamonds),
		card(domain.Nine, domain.Hearts),
		card(domain.Jack, domain.Hearts),
		card(domain.King, domain.Diamonds),
		card(domain.Ace, domain.Diamonds),
	}
	table := []domain.TablePair{{Attack: card(domain.Eight, domain.Clubs)}}
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:         domain.Seat(0),
			Phase:        domain.MatchPhaseDefense,
			Attacker:     domain.Seat(0),
			Defender:     domain.Seat(1),
			TrumpSuit:    domain.Spades,
			Table:        table,
			HandSizes:    []int{len(p1), len(p2)},
			StockCount:   19,
			DiscardCount: len(discard),
		},
		Hand: p1,
		PublicMemory: app.PublicCardMemory{
			Seat:       domain.Seat(0),
			Hand:       p1,
			Table:      table,
			Discard:    discard,
			KnownHeld:  [][]domain.Card{p1, p2},
			Seen:       slicesConcat(p1, p2, discard, []domain.Card{table[0].Attack}),
			HandSizes:  []int{len(p1), len(p2)},
			StockCount: 19,
			TrumpSuit:  domain.Spades,
		},
	}
	hidden := evaluation.BuildHiddenCards(&decision, nil)
	battle := evaluation.EvaluateBattleRiskForSeat(&decision, hidden, domain.Seat(1))
	if !almostEqual(battle.ContinueDefense, 1.4444444444) {
		t.Fatalf("ContinueDefense = %.6f, want spreadsheet cover cost", battle.ContinueDefense)
	}
	if !almostEqual(battle.TablePressure, 3) {
		t.Fatalf("TablePressure = %.6f, want spreadsheet pressure", battle.TablePressure)
	}
	components := evaluation.DefaultRiskModel().Components(&decision, hidden)
	if components[0].Score <= 0 || components[1].Score >= 0 {
		t.Fatalf("scores = P1 %d P2 %d, want P1 advantage like spreadsheet", components[0].Score, components[1].Score)
	}
	if math.Abs(float64(components[0].Score-748)) > 260 {
		t.Fatalf("P1 score = %d, want same broad band as spreadsheet 748", components[0].Score)
	}
}

func TestRiskModelBattleRiskIncludesExcelTablePressureRisk(t *testing.T) {
	defend := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Ten, domain.Clubs),
		AttackIndex: 0,
	}
	decision := battleDecision([]domain.Card{defend.Card}, []domain.TablePair{
		{Attack: card(domain.Nine, domain.Clubs)},
	}, []domain.Action{defend})
	decision.HandSizes = []int{1, 1}
	decision.PublicMemory = publicKnownHands(decision, [][]domain.Card{
		{card(domain.Nine, domain.Diamonds)},
		decision.Hand,
	})
	hidden := evaluation.BuildHiddenCards(&decision, nil)
	battle := evaluation.EvaluateBattleRisk(&decision, hidden)
	components := evaluation.DefaultRiskModel().Components(&decision, hidden)

	want := battle.Best + battle.TablePressure*0.3
	if !almostEqual(components[1].BattleRisk, want) {
		t.Fatalf("BattleRisk = %.6f, want best branch plus pressure %.6f",
			components[1].BattleRisk, want)
	}
}

func TestRiskModelOutletMatchesExcelAttackAndPairFormula(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      domain.MatchPhaseThrowIn,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{2, 2},
			StockCount: 8,
			Table: []domain.TablePair{
				{
					Attack:   card(domain.Six, domain.Spades),
					Defense:  card(domain.Seven, domain.Spades),
					Defended: true,
				},
			},
		},
		Hand: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Six, domain.Diamonds),
		},
	}
	components := evaluation.DefaultRiskModel().Components(
		&decision,
		evaluation.BuildHiddenCards(&decision, nil),
	)

	if !almostEqual(components[0].Outlet, 2.5) {
		t.Fatalf("Outlet = %.6f, want Excel B20 + B21*0.5 without B22 throw-in outlet",
			components[0].Outlet)
	}
}

func TestRiskModelDefenseStabilityUsesExcelTrumpAndHighCardFormula(t *testing.T) {
	cheap := defenseStabilityDecision(card(domain.Seven, domain.Clubs))
	trump := defenseStabilityDecision(card(domain.Ace, domain.Hearts))
	model := evaluation.DefaultRiskModel()

	cheapStability := model.Components(&cheap, evaluation.BuildHiddenCards(&cheap, nil))[0].DefenseStability
	trumpStability := model.Components(&trump, evaluation.BuildHiddenCards(&trump, nil))[0].DefenseStability

	if trumpStability <= cheapStability {
		t.Fatalf("cheap stability = %.2f, trump stability = %.2f; want Excel trump-weighted stability higher",
			cheapStability, trumpStability)
	}
}

func TestRiskModelInitiativeUsesExcelRoles(t *testing.T) {
	decision := initiativeDecision(domain.MatchPhaseThrowIn, 0, 1)
	model := evaluation.DefaultRiskModel()

	components := model.Components(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if components[0].Initiative != 1.5 {
		t.Fatalf("attacker initiative = %.2f, want Excel attacker bonus", components[0].Initiative)
	}
	if components[1].Initiative != -0.5 {
		t.Fatalf("defender initiative = %.2f, want Excel defender penalty", components[1].Initiative)
	}
	if components[2].Initiative != 0 {
		t.Fatalf("passive initiative = %.2f, want Excel neutral value", components[2].Initiative)
	}
}

func defenseStabilityDecision(handCard domain.Card) app.DecisionContext {
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      domain.MatchPhaseDefense,
			Attacker:   domain.Seat(1),
			Defender:   domain.Seat(0),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{1, 1},
			StockCount: 0,
			Table: []domain.TablePair{
				{Attack: card(domain.Six, domain.Clubs)},
			},
		},
		Hand: []domain.Card{handCard},
	}
}

func riskDecision(handSizes []int, stock int) app.DecisionContext {
	seat := domain.Seat(0)
	hand := make([]domain.Card, handSizes[int(seat)])
	for index := range hand {
		hand[index] = card(domain.Rank(int(domain.Six)+index%int(domain.Ace-domain.Six+1)), domain.Clubs)
	}
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       seat,
			Phase:      domain.MatchPhaseAttack,
			Attacker:   seat,
			Defender:   domain.Seat((int(seat) + 1) % len(handSizes)),
			TrumpSuit:  domain.Hearts,
			HandSizes:  handSizes,
			StockCount: stock,
		},
		Hand: hand,
	}
}

func initiativeDecision(phase domain.MatchPhase, attacker, defender int) app.DecisionContext {
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      phase,
			Attacker:   domain.Seat(attacker),
			Defender:   domain.Seat(defender),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{3, 3, 3},
			StockCount: 8,
		},
		Hand: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Seven, domain.Clubs),
			card(domain.Eight, domain.Clubs),
		},
	}
}

func almostEqual(got, want float64) bool {
	return math.Abs(got-want) <= 0.001
}

func slicesConcat(groups ...[]domain.Card) []domain.Card {
	var total int
	for _, group := range groups {
		total += len(group)
	}
	out := make([]domain.Card, 0, total)
	for _, group := range groups {
		out = append(out, group...)
	}
	return out
}
