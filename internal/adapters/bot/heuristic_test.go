package bot_test

import (
	"context"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestHeuristicControllerChoosesRankedLegalAction(t *testing.T) {
	highTrump := botCard(domain.Ace, domain.Hearts)
	throwIn := domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: highTrump}
	finish := domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)}
	turn := &app.TurnContext{
		DecisionContext: app.DecisionContext{
			SeatView: app.SeatView{
				Seat:       domain.Seat(0),
				Phase:      domain.MatchPhaseThrowIn,
				Attacker:   domain.Seat(0),
				Defender:   domain.Seat(1),
				TrumpSuit:  domain.Hearts,
				HandSizes:  []int{1, 5},
				StockCount: 8,
				Table: []domain.TablePair{
					{
						Attack:   botCard(domain.Six, domain.Clubs),
						Defense:  botCard(domain.Seven, domain.Clubs),
						Defended: true,
					},
				},
			},
			Hand: []domain.Card{highTrump},
			LegalActions: []domain.Action{
				throwIn,
				finish,
			},
		},
	}

	simpleAction, err := bot.NewSimpleStrategy().ChooseAction(context.Background(), &turn.DecisionContext)
	if err != nil {
		t.Fatalf("simple ChooseAction returned error: %v", err)
	}
	if simpleAction != throwIn {
		t.Fatalf("simple action = %+v, want throw-in setup", simpleAction)
	}

	decision, err := bot.NewHeuristicController().Decide(context.Background(), turn)
	if err != nil {
		t.Fatalf("heuristic Decide returned error: %v", err)
	}
	if decision.Kind != app.PlayerDecisionAction {
		t.Fatalf("heuristic decision = %+v, want action decision", decision)
	}
	if decision.Action != finish && decision.Action != throwIn {
		t.Fatalf("heuristic decision = %+v, want one legal action", decision)
	}
}

func botCard(rank domain.Rank, suit domain.Suit) domain.Card {
	return domain.Card{Rank: rank, Suit: suit}
}
