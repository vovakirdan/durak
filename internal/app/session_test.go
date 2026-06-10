package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSessionDecisionContextReturnsCopies(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	match := mustMatch(t, [][]domain.Card{
		{attack},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	})
	session := mustSession(t, match)

	decision := session.DecisionContext(domain.Seat(0))
	if !slices.Equal(decision.Hand, []domain.Card{attack}) {
		t.Fatalf("Hand = %v, want attacker card", decision.Hand)
	}
	if !slices.Equal(decision.HandSizes, []int{1, 1}) {
		t.Fatalf("HandSizes = %v, want [1 1]", decision.HandSizes)
	}
	if !slices.Contains(decision.LegalActions, domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: attack,
	}) {
		t.Fatalf("LegalActions = %v, want attack action", decision.LegalActions)
	}

	decision.Hand[0] = domain.Card{Rank: domain.Ace, Suit: domain.Spades}
	decision.HandSizes[0] = 99
	decision.LegalActions[0].Card = domain.Card{Rank: domain.Ace, Suit: domain.Spades}

	next := session.DecisionContext(domain.Seat(0))
	if !slices.Equal(next.Hand, []domain.Card{attack}) {
		t.Fatalf("mutating decision hand leaked into session: %v", next.Hand)
	}
	if !slices.Equal(next.HandSizes, []int{1, 1}) {
		t.Fatalf("mutating hand sizes leaked into session: %v", next.HandSizes)
	}
	if next.LegalActions[0].Card != attack {
		t.Fatalf("mutating legal actions leaked into session: %v", next.LegalActions)
	}
}

func TestSessionApplyStrategyAppliesChosenLegalAction(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{attack},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	strategy := strategyFunc(func(_ context.Context, decision *app.DecisionContext) (domain.Action, error) {
		return decision.LegalActions[0], nil
	})

	action, err := session.ApplyStrategy(context.Background(), domain.Seat(0), strategy)
	if err != nil {
		t.Fatalf("ApplyStrategy returned error: %v", err)
	}

	if action.Kind != domain.ActionKindAttack || action.Card != attack {
		t.Fatalf("action = %v, want attack with %v", action, attack)
	}
	if got := session.ViewForSeat(domain.Seat(0)).Phase; got != domain.MatchPhaseDefense {
		t.Fatalf("Phase = %v, want MatchPhaseDefense", got)
	}
}

func TestSessionApplyStrategyRejectsIllegalAction(t *testing.T) {
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	strategy := strategyFunc(func(context.Context, *app.DecisionContext) (domain.Action, error) {
		return domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}, nil
	})

	_, err := session.ApplyStrategy(context.Background(), domain.Seat(0), strategy)
	if !errors.Is(err, app.ErrIllegalAction) {
		t.Fatalf("ApplyStrategy error = %v, want ErrIllegalAction", err)
	}
	if got := session.ViewForSeat(domain.Seat(0)).Phase; got != domain.MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", got)
	}
}

func TestNewDealtSession(t *testing.T) {
	hands := [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Eight, Suit: domain.Hearts},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Queen, Suit: domain.Hearts},
			{Rank: domain.King, Suit: domain.Spades},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Hearts},
			{Rank: domain.Eight, Suit: domain.Spades},
		},
	}
	deck := deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))

	session, deal, err := app.NewDealtSession(2, domain.DefaultRuleProfile(), domain.DealOptions{
		Shuffler: domain.ShuffleFunc(func(cards []domain.Card) {
			copy(cards, deck)
		}),
	})
	if err != nil {
		t.Fatalf("NewDealtSession returned error: %v", err)
	}

	if session.ViewForSeat(domain.Seat(0)).TrumpSuit != domain.Hearts {
		t.Fatalf("TrumpSuit = %v, want Hearts", session.ViewForSeat(domain.Seat(0)).TrumpSuit)
	}
	if !slices.Equal(deal.Hands[0], hands[0]) {
		t.Fatalf("deal hand = %v, want %v", deal.Hands[0], hands[0])
	}
}

type strategyFunc func(context.Context, *app.DecisionContext) (domain.Action, error)

func (fn strategyFunc) ChooseAction(ctx context.Context, decision *app.DecisionContext) (domain.Action, error) {
	return fn(ctx, decision)
}

func mustSession(t *testing.T, match *domain.Match) *app.Session {
	t.Helper()
	session, err := app.NewSession(match)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	return session
}

func mustMatch(t *testing.T, hands [][]domain.Card) *domain.Match {
	t.Helper()
	deal := domain.InitialDeal{
		Hands:          hands,
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}
	match, err := domain.NewMatch(&deal, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	return match
}

func deckForDeal(hands [][]domain.Card, stock []domain.Card) []domain.Card {
	deck := make([]domain.Card, 0, 36)
	for cardIndex := range len(hands[0]) {
		for player := range hands {
			deck = append(deck, hands[player][cardIndex])
		}
	}
	deck = append(deck, stock...)
	return deck
}

func stockWithBottom(bottom domain.Card, hands ...[]domain.Card) []domain.Card {
	used := make(map[domain.Card]bool)
	for _, hand := range hands {
		for _, card := range hand {
			used[card] = true
		}
	}
	used[bottom] = true

	stock := make([]domain.Card, 0, 36)
	for _, card := range domain.NewDeck36() {
		if !used[card] {
			stock = append(stock, card)
		}
	}
	return append(stock, bottom)
}
