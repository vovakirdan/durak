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

func TestBattleRiskPrefersCheapDefenseOverTakingHeavyTable(t *testing.T) {
	defendClub := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Ace, domain.Clubs),
		AttackIndex: 0,
	}
	defendDiamond := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Six, domain.Hearts),
		AttackIndex: 1,
	}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := battleDecision([]domain.Card{defendClub.Card, defendDiamond.Card}, []domain.TablePair{
		{Attack: card(domain.King, domain.Clubs)},
		{Attack: card(domain.Ace, domain.Diamonds)},
	}, []domain.Action{defendClub, defendDiamond, take})

	risk := evaluation.EvaluateBattleRisk(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if risk.Best != risk.ContinueDefense {
		t.Fatalf("risk = %+v, want defending heavy table", risk)
	}
}

func TestDefenseCardRiskIncludesOpenedRankPressure(t *testing.T) {
	cheap := domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: card(domain.Nine, domain.Clubs), AttackIndex: 0}
	expensive := domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: card(domain.King, domain.Clubs), AttackIndex: 0}
	decision := battleDecision([]domain.Card{cheap.Card, expensive.Card}, []domain.TablePair{
		{Attack: card(domain.Eight, domain.Clubs)},
	}, []domain.Action{cheap, expensive})
	hidden := evaluation.BuildHiddenCards(&decision, nil)
	hidden.UnknownPool = []domain.Card{
		card(domain.Nine, domain.Diamonds),
		card(domain.Nine, domain.Hearts),
		card(domain.Nine, domain.Spades),
	}

	cheapRisk := evaluation.DefenseActionRisk(&decision, hidden, cheap)
	expensiveRisk := evaluation.DefenseActionRisk(&decision, hidden, expensive)

	if cheapRisk <= expensiveRisk {
		t.Fatalf("cheap risk = %.2f, expensive risk = %.2f; opened rank pressure should matter",
			cheapRisk, expensiveRisk)
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
