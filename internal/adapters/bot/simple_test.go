package bot_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSimpleStrategyDefendsBeforeTaking(t *testing.T) {
	strategy := bot.NewSimpleStrategy()
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := app.DecisionContext{
		SeatView: app.SeatView{TrumpSuit: domain.Hearts},
		LegalActions: []domain.Action{
			take,
			{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: defense},
		},
	}

	action, err := strategy.ChooseAction(context.Background(), &decision)
	if err != nil {
		t.Fatalf("ChooseAction returned error: %v", err)
	}
	if action.Kind != domain.ActionKindDefend || action.Card != defense {
		t.Fatalf("action = %v, want defend with %v", action, defense)
	}
}

func TestSimpleStrategyChoosesLowestNonTrumpCard(t *testing.T) {
	strategy := bot.NewSimpleStrategy()
	low := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	trump := domain.Card{Rank: domain.Seven, Suit: domain.Hearts}
	high := domain.Card{Rank: domain.Ace, Suit: domain.Spades}
	decision := app.DecisionContext{
		SeatView: app.SeatView{TrumpSuit: domain.Hearts},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: trump},
			{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: high},
			{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: low},
		},
	}

	action, err := strategy.ChooseAction(context.Background(), &decision)
	if err != nil {
		t.Fatalf("ChooseAction returned error: %v", err)
	}
	if action.Card != low {
		t.Fatalf("action = %v, want lowest non-trump %v", action, low)
	}
}

func TestSimpleStrategyThrowsInBeforeFinishingDefense(t *testing.T) {
	strategy := bot.NewSimpleStrategy()
	throwIn := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	decision := app.DecisionContext{
		SeatView: app.SeatView{TrumpSuit: domain.Hearts},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)},
			{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: throwIn},
		},
	}

	action, err := strategy.ChooseAction(context.Background(), &decision)
	if err != nil {
		t.Fatalf("ChooseAction returned error: %v", err)
	}
	if action.Kind != domain.ActionKindThrowIn || action.Card != throwIn {
		t.Fatalf("action = %v, want throw-in", action)
	}
}

func TestSimpleStrategyTransfersBeforeTaking(t *testing.T) {
	strategy := bot.NewSimpleStrategy()
	transfer := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	decision := app.DecisionContext{
		SeatView: app.SeatView{TrumpSuit: domain.Hearts},
		LegalActions: []domain.Action{
			{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
			{Kind: domain.ActionKindTransfer, Seat: domain.Seat(1), Card: transfer},
		},
	}

	action, err := strategy.ChooseAction(context.Background(), &decision)
	if err != nil {
		t.Fatalf("ChooseAction returned error: %v", err)
	}
	if action.Kind != domain.ActionKindTransfer || action.Card != transfer {
		t.Fatalf("action = %v, want transfer", action)
	}
}

func TestSimpleStrategyReturnsNoLegalAction(t *testing.T) {
	_, err := bot.NewSimpleStrategy().ChooseAction(context.Background(), &app.DecisionContext{})
	if !errors.Is(err, bot.ErrNoLegalAction) {
		t.Fatalf("ChooseAction error = %v, want ErrNoLegalAction", err)
	}
}

func TestSimpleStrategyHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := bot.NewSimpleStrategy().ChooseAction(ctx, &app.DecisionContext{
		LegalActions: []domain.Action{{Kind: domain.ActionKindFinishTake, Seat: domain.Seat(0)}},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ChooseAction error = %v, want context.Canceled", err)
	}
}
