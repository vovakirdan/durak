package app_test

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

var scriptedTrumpIndicator = domain.Card{Rank: domain.Nine, Suit: domain.Hearts}

func TestSeriesRunnerScriptedTakingAllowsThrowIns(t *testing.T) {
	attacks := []domain.Card{
		{Rank: domain.Six, Suit: domain.Clubs},
		{Rank: domain.Six, Suit: domain.Diamonds},
		{Rank: domain.Six, Suit: domain.Spades},
		{Rank: domain.Six, Suit: domain.Hearts},
	}
	hands := [][]domain.Card{
		{attacks[0], attacks[1], attacks[2], attacks[3], {Rank: domain.Ace, Suit: domain.Spades}},
		{
			{Rank: domain.Nine, Suit: domain.Diamonds},
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Clubs},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Ten, Suit: domain.Spades},
		},
	}

	result, events := runScriptedMatch(t, hands,
		scriptedAction(domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attacks[0]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[1]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[2]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[3]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindFinishTake, Seat: domain.Seat(0)}),
		scriptedConcede(domain.Seat(0)),
	)

	assertOneCompletedMatch(t, result)
	assertActionKinds(t, events, []domain.ActionKind{
		domain.ActionKindAttack,
		domain.ActionKindTake,
		domain.ActionKindThrowIn,
		domain.ActionKindThrowIn,
		domain.ActionKindThrowIn,
		domain.ActionKindFinishTake,
	})
}

func TestSeriesRunnerScriptedTransferAfterFirstRound(t *testing.T) {
	firstAttack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	secondAttack := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	firstRoundTransfer := domain.Card{Rank: domain.Six, Suit: domain.Diamonds}
	transfer := domain.Card{Rank: domain.Seven, Suit: domain.Diamonds}
	firstDefense := domain.Card{Rank: domain.Eight, Suit: domain.Clubs}
	secondDefense := domain.Card{Rank: domain.Eight, Suit: domain.Diamonds}
	firstAttackTransfer := domain.Action{Kind: domain.ActionKindTransfer, Seat: domain.Seat(1), Card: firstRoundTransfer}
	hands := [][]domain.Card{
		{firstAttack, secondAttack, firstDefense, secondDefense, {Rank: domain.Ten, Suit: domain.Spades}},
		{
			firstRoundTransfer,
			transfer,
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Clubs},
			{Rank: domain.Ace, Suit: domain.Clubs},
		},
	}

	result, events := runScriptedMatch(t, hands,
		scriptedAction(domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: firstAttack}),
		scriptedActionChecked(
			domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
			actionNotLegal(firstAttackTransfer),
		),
		scriptedAction(domain.Action{Kind: domain.ActionKindFinishTake, Seat: domain.Seat(0)}),
		scriptedAction(domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: secondAttack}),
		scriptedAction(domain.Action{Kind: domain.ActionKindTransfer, Seat: domain.Seat(1), Card: transfer}),
		scriptedAction(domain.Action{
			Kind:        domain.ActionKindDefend,
			Seat:        domain.Seat(0),
			Card:        firstDefense,
			AttackIndex: 0,
		}),
		scriptedAction(domain.Action{
			Kind:        domain.ActionKindDefend,
			Seat:        domain.Seat(0),
			Card:        secondDefense,
			AttackIndex: 1,
		}),
		scriptedAction(domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(1)}),
		scriptedConcede(domain.Seat(0)),
	)

	assertOneCompletedMatch(t, result)
	assertActionKinds(t, events, []domain.ActionKind{
		domain.ActionKindAttack,
		domain.ActionKindTake,
		domain.ActionKindFinishTake,
		domain.ActionKindAttack,
		domain.ActionKindTransfer,
		domain.ActionKindDefend,
		domain.ActionKindDefend,
		domain.ActionKindFinishDefense,
	})
}

func TestSeriesRunnerScriptedFirstDefenseLimit(t *testing.T) {
	attacks := []domain.Card{
		{Rank: domain.Six, Suit: domain.Clubs},
		{Rank: domain.Six, Suit: domain.Diamonds},
		{Rank: domain.Seven, Suit: domain.Spades},
		{Rank: domain.Six, Suit: domain.Spades},
		{Rank: domain.Eight, Suit: domain.Clubs},
	}
	defenses := []domain.Card{
		{Rank: domain.Seven, Suit: domain.Clubs},
		{Rank: domain.Eight, Suit: domain.Diamonds},
		{Rank: domain.Eight, Suit: domain.Spades},
		{Rank: domain.Nine, Suit: domain.Spades},
		{Rank: domain.Nine, Suit: domain.Clubs},
	}
	extraThrowIn := domain.Action{
		Kind: domain.ActionKindThrowIn,
		Seat: domain.Seat(0),
		Card: domain.Card{Rank: domain.Seven, Suit: domain.Diamonds},
	}
	hands := [][]domain.Card{
		{attacks[0], attacks[1], attacks[2], attacks[3], attacks[4], extraThrowIn.Card},
		{
			defenses[0],
			defenses[1],
			defenses[2],
			defenses[3],
			defenses[4],
			{Rank: domain.Queen, Suit: domain.Clubs},
		},
	}

	result, _ := runScriptedMatch(t, hands,
		scriptedAction(domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attacks[0]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: defenses[0], AttackIndex: 0}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[1]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: defenses[1], AttackIndex: 1}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[2]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: defenses[2], AttackIndex: 2}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[3]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: defenses[3], AttackIndex: 3}),
		scriptedAction(domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: attacks[4]}),
		scriptedAction(domain.Action{Kind: domain.ActionKindDefend, Seat: domain.Seat(1), Card: defenses[4], AttackIndex: 4}),
		scriptedActionChecked(
			domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)},
			actionNotLegal(extraThrowIn),
		),
		scriptedConcede(domain.Seat(1)),
	)

	assertOneCompletedMatch(t, result)
}

type scriptedTurnCheck func(*app.TurnContext) error

type scriptedStep struct {
	name   string
	seat   domain.Seat
	choose func(*app.TurnContext) (app.PlayerDecision, error)
}

type scriptedMatchScript struct {
	steps []scriptedStep
	next  int
}

type scriptedController struct {
	script *scriptedMatchScript
	seat   domain.Seat
}

func runScriptedMatch(
	t *testing.T,
	hands [][]domain.Card,
	steps ...scriptedStep,
) (app.SeriesRunResult, []app.Event) {
	t.Helper()
	store := app.NewInMemoryEventStore()
	script := &scriptedMatchScript{steps: steps}
	runner := mustRunner(t, app.SeriesRunnerOptions{
		Series:             mustScriptedSeries(t, len(hands[0])),
		Controllers:        script.controllers(),
		Deal:               fixedDeck(deckForDeal(hands, stockWithBottom(scriptedTrumpIndicator, hands...))),
		EventStore:         store,
		BaseMatchID:        "scripted-match",
		MaxActionsPerMatch: len(steps) + 5,
	})

	result, err := runner.Run(context.Background(), 1)
	if err != nil {
		t.Fatalf("Run returned error after %d/%d scripted steps: %v", script.next, len(script.steps), err)
	}
	script.mustExhausted(t)
	return result, store.EventsForMatch("scripted-match")
}

func mustScriptedSeries(t *testing.T, handSize int) *app.Series {
	t.Helper()
	profile := domain.DefaultRuleProfile()
	profile.InitialHandSize = handSize
	profile.RedealSameSuitThreshold = handSize + 1
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "scripted-series",
		Seats:    []domain.Seat{0, 1},
		Profile:  profile,
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	return series
}

func (s *scriptedMatchScript) controllers() map[domain.Seat]app.PlayerController {
	return map[domain.Seat]app.PlayerController{
		domain.Seat(0): scriptedController{script: s, seat: domain.Seat(0)},
		domain.Seat(1): scriptedController{script: s, seat: domain.Seat(1)},
	}
}

func (s *scriptedMatchScript) decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	if err := ctx.Err(); err != nil {
		return app.PlayerDecision{}, err
	}
	if turn == nil {
		return app.PlayerDecision{}, app.ErrNilTurn
	}
	if s.next >= len(s.steps) {
		return app.PlayerDecision{}, fmt.Errorf("script exhausted at turn %d for seat %d", turn.TurnNumber, turn.Seat)
	}
	step := s.steps[s.next]
	if step.seat != turn.Seat {
		return app.PlayerDecision{}, fmt.Errorf("script step %d %q got seat %d, want %d", s.next+1, step.name, turn.Seat, step.seat)
	}
	decision, err := step.choose(turn)
	if err != nil {
		return app.PlayerDecision{}, fmt.Errorf("script step %d %q: %w", s.next+1, step.name, err)
	}
	s.next++
	return decision, nil
}

func (s *scriptedMatchScript) mustExhausted(t *testing.T) {
	t.Helper()
	if s.next != len(s.steps) {
		t.Fatalf("script used %d/%d steps", s.next, len(s.steps))
	}
}

func (c scriptedController) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	if turn != nil && turn.Seat != c.seat {
		return app.PlayerDecision{}, fmt.Errorf("controller for seat %d received turn for seat %d", c.seat, turn.Seat)
	}
	return c.script.decide(ctx, turn)
}

func scriptedAction(action domain.Action) scriptedStep {
	return scriptedActionChecked(action)
}

func scriptedActionChecked(action domain.Action, checks ...scriptedTurnCheck) scriptedStep {
	return scriptedStep{
		name: fmt.Sprintf("%v %s", action.Kind, action.Card),
		seat: action.Seat,
		choose: func(turn *app.TurnContext) (app.PlayerDecision, error) {
			for _, check := range checks {
				if err := check(turn); err != nil {
					return app.PlayerDecision{}, err
				}
			}
			if !slices.Contains(turn.LegalActions, action) {
				return app.PlayerDecision{}, fmt.Errorf("action %+v is not legal; legal actions: %+v", action, turn.LegalActions)
			}
			return app.ActionDecision(action), nil
		},
	}
}

func scriptedConcede(seat domain.Seat) scriptedStep {
	return scriptedStep{
		name: "concede",
		seat: seat,
		choose: func(turn *app.TurnContext) (app.PlayerDecision, error) {
			if !turn.CanConcede {
				return app.PlayerDecision{}, fmt.Errorf("seat %d cannot concede", seat)
			}
			return app.ConcedeDecision(), nil
		},
	}
}

func actionNotLegal(action domain.Action) scriptedTurnCheck {
	return func(turn *app.TurnContext) error {
		if slices.Contains(turn.LegalActions, action) {
			return fmt.Errorf("action %+v is legal, want it absent", action)
		}
		return nil
	}
}

func assertOneCompletedMatch(t *testing.T, result app.SeriesRunResult) {
	t.Helper()
	if len(result.Matches) != 1 {
		t.Fatalf("matches = %+v, want one completed match", result.Matches)
	}
	if result.Matches[0].Winner == domain.NoSeat || result.Matches[0].Loser == domain.NoSeat {
		t.Fatalf("match result = %+v, want concession winner and loser", result.Matches[0])
	}
}

func assertActionKinds(t *testing.T, events []app.Event, want []domain.ActionKind) {
	t.Helper()
	var got []domain.ActionKind
	for _, event := range events {
		if event.Domain.Action != nil {
			got = append(got, event.Domain.Action.Action.Kind)
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("action kinds = %v, want %v", got, want)
	}
}
