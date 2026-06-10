package domain_test

import (
	"errors"
	"slices"
	"testing"

	. "github.com/vovakirdan/durak/internal/domain"
)

func TestDealInitial(t *testing.T) {
	profile := DefaultRuleProfile()
	hands := [][]Card{
		{
			{Rank: Six, Suit: Clubs},
			{Rank: Ten, Suit: Hearts},
			{Rank: Seven, Suit: Diamonds},
			{Rank: Jack, Suit: Clubs},
			{Rank: Queen, Suit: Diamonds},
			{Rank: Nine, Suit: Spades},
		},
		{
			{Rank: Eight, Suit: Hearts},
			{Rank: Six, Suit: Diamonds},
			{Rank: King, Suit: Clubs},
			{Rank: Ten, Suit: Spades},
			{Rank: Jack, Suit: Diamonds},
			{Rank: Queen, Suit: Clubs},
		},
	}
	stock := stockWithBottom(Card{Rank: Nine, Suit: Hearts}, hands...)
	deck := deckForDeal(hands, stock)

	deal, err := DealInitial(2, profile, DealOptions{Shuffler: copyDeck(deck)})
	if err != nil {
		t.Fatalf("DealInitial returned error: %v", err)
	}

	if !equalHands(deal.Hands, hands) {
		t.Fatalf("hands = %v, want %v", deal.Hands, hands)
	}
	if deal.TrumpIndicator != (Card{Rank: Nine, Suit: Hearts}) {
		t.Fatalf("TrumpIndicator = %v, want 9H", deal.TrumpIndicator)
	}
	if deal.TrumpSuit != Hearts {
		t.Fatalf("TrumpSuit = %v, want Hearts", deal.TrumpSuit)
	}
	if deal.FirstAttacker != 1 {
		t.Fatalf("FirstAttacker = %d, want 1", deal.FirstAttacker)
	}
	if deal.Redeals != 0 {
		t.Fatalf("Redeals = %d, want 0", deal.Redeals)
	}
	if deal.TrumpReselections != 0 {
		t.Fatalf("TrumpReselections = %d, want 0", deal.TrumpReselections)
	}
}

func TestDealInitialRedealsWhenHandHasTooManyCardsOfOneSuit(t *testing.T) {
	profile := DefaultRuleProfile()
	invalidHands := [][]Card{
		{
			{Rank: Six, Suit: Clubs},
			{Rank: Seven, Suit: Clubs},
			{Rank: Eight, Suit: Clubs},
			{Rank: Nine, Suit: Clubs},
			{Rank: Ten, Suit: Clubs},
			{Rank: Six, Suit: Spades},
		},
		{
			{Rank: Six, Suit: Diamonds},
			{Rank: Seven, Suit: Diamonds},
			{Rank: Eight, Suit: Hearts},
			{Rank: Nine, Suit: Hearts},
			{Rank: Ten, Suit: Spades},
			{Rank: Jack, Suit: Spades},
		},
	}
	validHands := [][]Card{
		{
			{Rank: Six, Suit: Clubs},
			{Rank: Seven, Suit: Diamonds},
			{Rank: Eight, Suit: Hearts},
			{Rank: Nine, Suit: Spades},
			{Rank: Ten, Suit: Clubs},
			{Rank: Jack, Suit: Diamonds},
		},
		{
			{Rank: Queen, Suit: Hearts},
			{Rank: King, Suit: Spades},
			{Rank: Ace, Suit: Clubs},
			{Rank: Six, Suit: Diamonds},
			{Rank: Seven, Suit: Hearts},
			{Rank: Eight, Suit: Spades},
		},
	}
	decks := [][]Card{
		deckForDeal(invalidHands, stockWithBottom(Card{Rank: Nine, Suit: Hearts}, invalidHands...)),
		deckForDeal(validHands, stockWithBottom(Card{Rank: Ten, Suit: Hearts}, validHands...)),
	}

	deal, err := DealInitial(2, profile, DealOptions{Shuffler: copyDeckSequence(decks)})
	if err != nil {
		t.Fatalf("DealInitial returned error: %v", err)
	}

	if deal.Redeals != 1 {
		t.Fatalf("Redeals = %d, want 1", deal.Redeals)
	}
	if !equalHands(deal.Hands, validHands) {
		t.Fatalf("hands = %v, want %v", deal.Hands, validHands)
	}
}

func TestDealInitialReshufflesOnlyStockWhenTrumpIndicatorIsAce(t *testing.T) {
	profile := DefaultRuleProfile()
	hands := [][]Card{
		{
			{Rank: Six, Suit: Clubs},
			{Rank: Seven, Suit: Diamonds},
			{Rank: Eight, Suit: Hearts},
			{Rank: Nine, Suit: Spades},
			{Rank: Ten, Suit: Clubs},
			{Rank: Jack, Suit: Diamonds},
		},
		{
			{Rank: Queen, Suit: Hearts},
			{Rank: King, Suit: Spades},
			{Rank: Ace, Suit: Clubs},
			{Rank: Six, Suit: Diamonds},
			{Rank: Seven, Suit: Hearts},
			{Rank: Eight, Suit: Spades},
		},
	}
	initialStock := stockWithBottom(Card{Rank: Ace, Suit: Hearts}, hands...)
	reshuffledStock := slices.Clone(initialStock)
	moveCardToBottom(reshuffledStock, Card{Rank: Nine, Suit: Hearts})
	initialDeck := deckForDeal(hands, initialStock)

	deal, err := DealInitial(2, profile, DealOptions{
		Shuffler: ShuffleFunc(func(cards []Card) {
			switch len(cards) {
			case 36:
				copy(cards, initialDeck)
			default:
				copy(cards, reshuffledStock)
			}
		}),
	})
	if err != nil {
		t.Fatalf("DealInitial returned error: %v", err)
	}

	if !equalHands(deal.Hands, hands) {
		t.Fatalf("hands changed after trump reselection: %v", deal.Hands)
	}
	if deal.TrumpIndicator != (Card{Rank: Nine, Suit: Hearts}) {
		t.Fatalf("TrumpIndicator = %v, want 9H", deal.TrumpIndicator)
	}
	if deal.TrumpReselections != 1 {
		t.Fatalf("TrumpReselections = %d, want 1", deal.TrumpReselections)
	}
}

func TestDealInitialRandomFirstAttackerWhenNoPlayerHasTrump(t *testing.T) {
	profile := DefaultRuleProfile()
	hands := [][]Card{
		{
			{Rank: Six, Suit: Clubs},
			{Rank: Seven, Suit: Clubs},
			{Rank: Eight, Suit: Diamonds},
			{Rank: Nine, Suit: Diamonds},
			{Rank: Ten, Suit: Spades},
			{Rank: Jack, Suit: Spades},
		},
		{
			{Rank: Queen, Suit: Clubs},
			{Rank: King, Suit: Clubs},
			{Rank: Ace, Suit: Diamonds},
			{Rank: Six, Suit: Diamonds},
			{Rank: Seven, Suit: Spades},
			{Rank: Eight, Suit: Spades},
		},
	}
	stock := stockWithBottom(Card{Rank: Nine, Suit: Hearts}, hands...)
	deck := deckForDeal(hands, stock)

	deal, err := DealInitial(2, profile, DealOptions{
		Shuffler: copyDeck(deck),
		Choose: func(n int) int {
			if n != 2 {
				t.Fatalf("Choose called with n = %d, want 2", n)
			}
			return 1
		},
	})
	if err != nil {
		t.Fatalf("DealInitial returned error: %v", err)
	}

	if deal.FirstAttacker != 1 {
		t.Fatalf("FirstAttacker = %d, want 1", deal.FirstAttacker)
	}
	if !deal.RandomFirstAttacker {
		t.Fatal("RandomFirstAttacker = false, want true")
	}
}

func TestDealInitialRejectsUnsupportedPlayerCount(t *testing.T) {
	_, err := DealInitial(6, DefaultRuleProfile(), DealOptions{})
	if !errors.Is(err, ErrInvalidPlayerCount) {
		t.Fatalf("DealInitial error = %v, want ErrInvalidPlayerCount", err)
	}
}

func copyDeck(deck []Card) ShuffleFunc {
	return func(cards []Card) {
		copy(cards, deck)
	}
}

func copyDeckSequence(decks [][]Card) ShuffleFunc {
	next := 0
	return func(cards []Card) {
		if next >= len(decks) {
			copy(cards, decks[len(decks)-1])
			return
		}
		copy(cards, decks[next])
		next++
	}
}

func deckForDeal(hands [][]Card, stock []Card) []Card {
	deck := make([]Card, 0, 36)
	for cardIndex := range len(hands[0]) {
		for player := range hands {
			deck = append(deck, hands[player][cardIndex])
		}
	}
	deck = append(deck, stock...)
	return deck
}

func stockWithBottom(bottom Card, hands ...[]Card) []Card {
	used := make(map[Card]bool)
	for _, hand := range hands {
		for _, card := range hand {
			used[card] = true
		}
	}
	used[bottom] = true

	stock := make([]Card, 0, 36)
	for _, card := range NewDeck36() {
		if !used[card] {
			stock = append(stock, card)
		}
	}
	stock = append(stock, bottom)
	return stock
}

func moveCardToBottom(cards []Card, card Card) {
	index := slices.Index(cards, card)
	if index == -1 {
		return
	}
	copy(cards[index:], cards[index+1:])
	cards[len(cards)-1] = card
}

func equalHands(a, b [][]Card) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
