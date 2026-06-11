package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestStrategyControllerWrapsStrategy(t *testing.T) {
	action := domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs},
	}
	controller := app.StrategyController{
		Strategy: strategyFunc(func(_ context.Context, decision *app.DecisionContext) (domain.Action, error) {
			if len(decision.LegalActions) != 1 {
				t.Fatalf("LegalActions = %v, want one action", decision.LegalActions)
			}
			return decision.LegalActions[0], nil
		}),
	}

	decision, err := controller.Decide(context.Background(), &app.TurnContext{
		DecisionContext: app.DecisionContext{LegalActions: []domain.Action{action}},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Kind != app.PlayerDecisionAction || decision.Action != action {
		t.Fatalf("decision = %+v, want action decision", decision)
	}
}

func TestSeriesRunnerPlaysConsecutiveMatches(t *testing.T) {
	store := app.NewInMemoryEventStore()
	series := mustRunnerSeries(t)
	hands := runnerHands()
	runner := mustRunner(t, app.SeriesRunnerOptions{
		Series:      series,
		Controllers: concedeControllers(),
		Deal:        fixedDeck(deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))),
		EventStore:  store,
	})

	result, err := runner.Run(context.Background(), 2)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.Matches) != 2 {
		t.Fatalf("matches = %+v, want two results", result.Matches)
	}
	if len(result.Turns) != 2 {
		t.Fatalf("turns = %+v, want one concession per match", result.Turns)
	}
	if result.Turns[0].Seat != domain.Seat(0) || result.Turns[1].Seat != domain.Seat(1) {
		t.Fatalf("turn seats = %d/%d, want 0/1", result.Turns[0].Seat, result.Turns[1].Seat)
	}
	secondEvents := store.EventsForMatch("runner-match-2")
	if len(secondEvents) < 2 || secondEvents[1].Domain.Deal == nil {
		t.Fatalf("second events = %+v, want deal", secondEvents)
	}
	if secondEvents[1].Domain.Deal.FirstAttacker != domain.Seat(1) {
		t.Fatalf("second first attacker = %d, want previous-loser override to seat 1", secondEvents[1].Domain.Deal.FirstAttacker)
	}
}

func TestSeriesRunnerStopsAtActionLimit(t *testing.T) {
	hands := runnerHands()
	runner := mustRunner(t, app.SeriesRunnerOptions{
		Series:             mustRunnerSeries(t),
		Controllers:        firstLegalControllers(),
		Deal:               fixedDeck(deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))),
		MaxActionsPerMatch: 1,
	})

	result, err := runner.Run(context.Background(), 1)
	if !errors.Is(err, app.ErrActionLimitExceeded) {
		t.Fatalf("Run error = %v, want ErrActionLimitExceeded", err)
	}
	if len(result.Turns) != 1 {
		t.Fatalf("turns = %+v, want one accepted action before limit", result.Turns)
	}
}

func TestSeriesRunnerRejectsMissingController(t *testing.T) {
	_, err := app.NewSeriesRunner(&app.SeriesRunnerOptions{
		Series: mustRunnerSeries(t),
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(0): controllerFunc(func(context.Context, *app.TurnContext) (app.PlayerDecision, error) {
				return app.ConcedeDecision(), nil
			}),
		},
	})
	if !errors.Is(err, app.ErrMissingPlayerController) {
		t.Fatalf("NewSeriesRunner error = %v, want ErrMissingPlayerController", err)
	}
}

func TestSeriesRunnerRejectsMutatedLegalActions(t *testing.T) {
	hands := runnerHands()
	runner := mustRunner(t, app.SeriesRunnerOptions{
		Series: mustRunnerSeries(t),
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(0): controllerFunc(func(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
				illegal := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
				turn.LegalActions = append(turn.LegalActions, illegal)
				return app.ActionDecision(illegal), nil
			}),
			domain.Seat(1): controllerFunc(func(context.Context, *app.TurnContext) (app.PlayerDecision, error) {
				return app.ConcedeDecision(), nil
			}),
		},
		Deal: fixedDeck(deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))),
	})

	_, err := runner.Run(context.Background(), 1)
	if !errors.Is(err, app.ErrIllegalAction) {
		t.Fatalf("Run error = %v, want ErrIllegalAction", err)
	}
}

type controllerFunc func(context.Context, *app.TurnContext) (app.PlayerDecision, error)

func (fn controllerFunc) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	return fn(ctx, turn)
}

func mustRunner(t *testing.T, options app.SeriesRunnerOptions) *app.SeriesRunner {
	t.Helper()
	runner, err := app.NewSeriesRunner(&options)
	if err != nil {
		t.Fatalf("NewSeriesRunner returned error: %v", err)
	}
	return runner
}

func mustRunnerSeries(t *testing.T) *app.Series {
	t.Helper()
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "runner-series",
		Seats:    []domain.Seat{0, 1},
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	return series
}

func concedeControllers() map[domain.Seat]app.PlayerController {
	return map[domain.Seat]app.PlayerController{
		domain.Seat(0): controllerFunc(func(context.Context, *app.TurnContext) (app.PlayerDecision, error) {
			return app.ConcedeDecision(), nil
		}),
		domain.Seat(1): controllerFunc(func(context.Context, *app.TurnContext) (app.PlayerDecision, error) {
			return app.ConcedeDecision(), nil
		}),
	}
}

func firstLegalControllers() map[domain.Seat]app.PlayerController {
	controller := controllerFunc(func(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
		return app.ActionDecision(turn.LegalActions[0]), nil
	})
	return map[domain.Seat]app.PlayerController{
		domain.Seat(0): controller,
		domain.Seat(1): controller,
	}
}

func runnerHands() [][]domain.Card {
	return [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Hearts},
			{Rank: domain.Eight, Suit: domain.Diamonds},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.Ten, Suit: domain.Hearts},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Spades},
			{Rank: domain.Eight, Suit: domain.Spades},
		},
	}
}
