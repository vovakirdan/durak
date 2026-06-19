package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestBattleRiskCanPreferTakingOverBurningLastTrump(t *testing.T) {
	trumpAce := card(domain.Ace, domain.Hearts)
	defend := domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: trumpAce, AttackIndex: 0}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := battleDecision([]domain.Card{trumpAce}, []domain.TablePair{
		{Attack: card(domain.Six, domain.Clubs)},
	}, []domain.Action{defend, take})

	risk := evaluation.EvaluateBattleRisk(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if risk.Best != risk.TakeNow {
		t.Fatalf("risk = %+v, want taking cheap card over burning last trump", risk)
	}
}

func TestBattleRiskPrefersCheapDefenseOverTakingCostlyTable(t *testing.T) {
	defendClub := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Nine, domain.Clubs),
		AttackIndex: 0,
	}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := battleDecision([]domain.Card{defendClub.Card}, []domain.TablePair{
		{Attack: card(domain.Eight, domain.Clubs)},
	}, []domain.Action{defendClub, take})

	risk := evaluation.EvaluateBattleRisk(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if risk.Best != risk.ContinueDefense {
		t.Fatalf("risk = %+v, want defending costly table", risk)
	}
}

func TestDefenseCardRiskIncludesOpenedRankPressure(t *testing.T) {
	cheap := domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: card(domain.Nine, domain.Clubs), AttackIndex: 0}
	expensive := domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: card(domain.King, domain.Clubs), AttackIndex: 0}
	decision := battleDecision([]domain.Card{cheap.Card, expensive.Card}, []domain.TablePair{
		{Attack: card(domain.Eight, domain.Clubs)},
	}, []domain.Action{cheap, expensive})
	decision.PublicMemory = publicKnownHands(decision, [][]domain.Card{
		{
			card(domain.Nine, domain.Diamonds),
			card(domain.Nine, domain.Hearts),
		},
		decision.Hand,
	})
	decision.PublicMemory.Discard = []domain.Card{
		card(domain.King, domain.Diamonds),
		card(domain.King, domain.Hearts),
		card(domain.King, domain.Spades),
	}
	hidden := evaluation.BuildHiddenCards(&decision, nil)

	cheapRisk := evaluation.DefenseActionRisk(&decision, hidden, cheap)
	expensiveRisk := evaluation.DefenseActionRisk(&decision, hidden, expensive)

	if cheapRisk <= expensiveRisk {
		t.Fatalf("cheap risk = %.2f, expensive risk = %.2f; opened rank pressure should matter",
			cheapRisk, expensiveRisk)
	}
}

func TestBattleRiskTablePressureUsesExcelFreeRankCount(t *testing.T) {
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

	risk := evaluation.EvaluateBattleRisk(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if !almostEqual(risk.TablePressure, 3) {
		t.Fatalf("TablePressure = %.6f, want free rank count", risk.TablePressure)
	}
}

func battleDecision(hand []domain.Card, table []domain.TablePair, actions []domain.Action) app.DecisionContext {
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(1),
			Phase:      domain.MatchPhaseDefense,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{4, len(hand)},
			StockCount: 12,
			Table:      table,
		},
		Hand:         hand,
		LegalActions: actions,
	}
}
