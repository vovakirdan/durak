package bot_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRandomLegalControllerChoosesIndexedLegalAction(t *testing.T) {
	actions := []domain.Action{
		{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
		{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs}},
	}
	controller := bot.NewRandomLegalController(func(n int) int {
		if n != len(actions) {
			t.Fatalf("n = %d, want %d", n, len(actions))
		}
		return 1
	})

	decision, err := controller.Decide(context.Background(), &app.TurnContext{
		DecisionContext: app.DecisionContext{LegalActions: actions},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Kind != app.PlayerDecisionAction || decision.Action != actions[1] {
		t.Fatalf("decision = %+v, want second action", decision)
	}
}

func TestRandomLegalControllerRejectsNoLegalActions(t *testing.T) {
	controller := bot.NewRandomLegalController(func(int) int {
		t.Fatal("chooser should not be called")
		return 0
	})

	_, err := controller.Decide(context.Background(), &app.TurnContext{})
	if !errors.Is(err, bot.ErrNoLegalAction) {
		t.Fatalf("Decide error = %v, want ErrNoLegalAction", err)
	}
}

func TestRandomLegalControllerRejectsInvalidChoice(t *testing.T) {
	controller := bot.NewRandomLegalController(func(n int) int {
		return n
	})
	_, err := controller.Decide(context.Background(), &app.TurnContext{
		DecisionContext: app.DecisionContext{
			LegalActions: []domain.Action{{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}},
		},
	})
	if !errors.Is(err, app.ErrInvalidPlayerDecision) {
		t.Fatalf("Decide error = %v, want ErrInvalidPlayerDecision", err)
	}
}

func TestRandomLegalControllerHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	controller := bot.NewRandomLegalController(func(int) int {
		t.Fatal("chooser should not be called")
		return 0
	})
	_, err := controller.Decide(ctx, &app.TurnContext{
		DecisionContext: app.DecisionContext{
			LegalActions: []domain.Action{{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}},
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Decide error = %v, want context.Canceled", err)
	}
}
