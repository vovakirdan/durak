package bot_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestNewControllerDefaultsToSimple(t *testing.T) {
	controller, err := bot.NewController(bot.ControllerSpec{}, domain.Seat(1))
	if err != nil {
		t.Fatalf("NewController returned error: %v", err)
	}
	decision := controllerDecision(t, controller, []domain.Action{
		{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
		{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs}},
	})

	if decision.Action.Kind != domain.ActionKindDefend {
		t.Fatalf("decision = %+v, want simple defend priority", decision)
	}
}

func TestNewControllerCreatesSeededRandom(t *testing.T) {
	controller, err := bot.NewController(bot.ControllerSpec{
		Kind:   bot.ControllerRandom,
		Seed:   42,
		Seeded: true,
	}, domain.Seat(1))
	if err != nil {
		t.Fatalf("NewController returned error: %v", err)
	}
	actions := []domain.Action{
		{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
		{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs}},
	}

	decision := controllerDecision(t, controller, actions)

	if decision.Kind != app.PlayerDecisionAction {
		t.Fatalf("decision = %+v, want action decision", decision)
	}
	if decision.Action != actions[0] && decision.Action != actions[1] {
		t.Fatalf("decision action = %+v, want one legal action", decision.Action)
	}
}

func TestNewControllerCreatesHeuristic(t *testing.T) {
	controller, err := bot.NewController(bot.ControllerSpec{Kind: bot.ControllerHeuristic}, domain.Seat(1))
	if err != nil {
		t.Fatalf("NewController returned error: %v", err)
	}
	decision := controllerDecision(t, controller, []domain.Action{
		{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
		{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs}},
	})

	if decision.Action.Kind != domain.ActionKindDefend {
		t.Fatalf("decision = %+v, want heuristic defend priority", decision)
	}
}

func TestNewControllerRejectsUnknownKind(t *testing.T) {
	_, err := bot.NewController(bot.ControllerSpec{Kind: "unknown"}, domain.Seat(0))
	if !errors.Is(err, bot.ErrUnknownController) {
		t.Fatalf("NewController error = %v, want ErrUnknownController", err)
	}
}

func TestNewControllerRejectsUnsupportedRandomSeat(t *testing.T) {
	_, err := bot.NewController(bot.ControllerSpec{
		Kind:   bot.ControllerRandom,
		Seeded: true,
	}, domain.Seat(6))
	if !errors.Is(err, bot.ErrUnsupportedControllerSeat) {
		t.Fatalf("NewController error = %v, want ErrUnsupportedControllerSeat", err)
	}
}

func controllerDecision(t *testing.T, controller app.PlayerController, actions []domain.Action) app.PlayerDecision {
	t.Helper()
	seatView := app.SeatView{TrumpSuit: domain.Hearts}
	hand := []domain.Card(nil)
	for _, action := range actions {
		if action.Kind == domain.ActionKindDefend {
			seatView.Seat = action.Seat
			seatView.Phase = domain.MatchPhaseDefense
			seatView.Defender = action.Seat
			seatView.Attacker = domain.Seat(0)
			seatView.HandSizes = []int{1, 2}
			seatView.Table = []domain.TablePair{{Attack: domain.Card{Rank: domain.Six, Suit: domain.Clubs}}}
			hand = append(hand, action.Card)
		}
	}
	decision, err := controller.Decide(context.Background(), &app.TurnContext{
		DecisionContext: app.DecisionContext{
			SeatView:     seatView,
			Hand:         hand,
			LegalActions: actions,
		},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	return decision
}
