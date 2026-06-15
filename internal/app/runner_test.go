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

func TestSeriesRunnerPlaysThreeSeatMatch(t *testing.T) {
	series := mustRunnerSeriesWithSeats(t, []domain.Seat{0, 1, 2})
	runner := mustRunner(t, app.SeriesRunnerOptions{
		Series:      series,
		Controllers: concedeControllersForSeats(series.Seats()),
		Deal:        domain.SeededDealOptions(42),
	})

	result, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.Matches) != 1 || result.Matches[0].Loser == domain.NoSeat {
		t.Fatalf("matches = %+v, want one decided match", result.Matches)
	}
	if len(result.Turns) != 1 || result.Turns[0].Seat < 0 || result.Turns[0].Seat > 2 {
		t.Fatalf("turns = %+v, want one three-seat concession", result.Turns)
	}
}

func TestSeriesRunnerPollsEligibleThrowInSeat(t *testing.T) {
	hands := [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Hearts},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Clubs},
			{Rank: domain.Queen, Suit: domain.Diamonds},
			{Rank: domain.King, Suit: domain.Diamonds},
			{Rank: domain.Ace, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Seven, Suit: domain.Hearts},
			{Rank: domain.Eight, Suit: domain.Diamonds},
			{Rank: domain.Ten, Suit: domain.Spades},
			{Rank: domain.Jack, Suit: domain.Spades},
			{Rank: domain.Queen, Suit: domain.Spades},
			{Rank: domain.King, Suit: domain.Spades},
		},
		{
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Ten, Suit: domain.Hearts},
			{Rank: domain.Jack, Suit: domain.Hearts},
			{Rank: domain.Queen, Suit: domain.Hearts},
			{Rank: domain.King, Suit: domain.Hearts},
			{Rank: domain.Ace, Suit: domain.Spades},
		},
	}
	seat2Threw := false
	controller := controllerFunc(func(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
		if seat2Threw {
			return app.ConcedeDecision(), nil
		}
		for _, action := range turn.LegalActions {
			if action.Kind == domain.ActionKindThrowIn && action.Seat == domain.Seat(2) {
				seat2Threw = true
				return app.ActionDecision(action), nil
			}
		}
		return app.ActionDecision(turn.LegalActions[0]), nil
	})
	runner := mustRunner(t, app.SeriesRunnerOptions{
		Series: mustRunnerSeriesWithSeats(t, []domain.Seat{0, 1, 2}),
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(0): controller,
			domain.Seat(1): controller,
			domain.Seat(2): controller,
		},
		Deal: fixedDeck(deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))),
	})

	result, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	for _, turn := range result.Turns {
		if turn.Seat == domain.Seat(2) && turn.Decision.Action.Kind == domain.ActionKindThrowIn {
			return
		}
	}
	t.Fatalf("turns = %+v, want seat2 throw-in turn", result.Turns)
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
	return mustRunnerSeriesWithSeats(t, []domain.Seat{0, 1})
}

func mustRunnerSeriesWithSeats(t *testing.T, seats []domain.Seat) *app.Series {
	t.Helper()
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "runner-series",
		Seats:    seats,
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	return series
}

func concedeControllers() map[domain.Seat]app.PlayerController {
	return concedeControllersForSeats([]domain.Seat{0, 1})
}

func concedeControllersForSeats(seats []domain.Seat) map[domain.Seat]app.PlayerController {
	controllers := make(map[domain.Seat]app.PlayerController, len(seats))
	for _, seat := range seats {
		controllers[seat] = controllerFunc(func(context.Context, *app.TurnContext) (app.PlayerDecision, error) {
			return app.ConcedeDecision(), nil
		})
	}
	return controllers
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
