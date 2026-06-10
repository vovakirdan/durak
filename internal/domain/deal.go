package domain

import (
	"errors"
	"fmt"
	"slices"
)

var (
	ErrInvalidPlayerCount = errors.New("invalid player count")
	ErrSetupAttemptsLimit = errors.New("setup attempts limit reached")
)

// ShuffleFunc mutates cards into a shuffled order.
type ShuffleFunc func(cards []Card)

// IntnFunc returns an integer in [0, n).
type IntnFunc func(n int) int

// DealOptions contains injectable randomness for deterministic tests.
type DealOptions struct {
	Shuffle ShuffleFunc
	Choose  IntnFunc
}

// InitialDeal is the result of initial dealing and trump selection.
type InitialDeal struct {
	Hands               [][]Card
	Stock               []Card
	TrumpIndicator      Card
	TrumpSuit           Suit
	FirstAttacker       int
	Redeals             int
	TrumpReselections   int
	RandomFirstAttacker bool
}

// DealInitial deals initial hands, selects a valid trump indicator, and picks
// the first attacker. It implements only the setup part of the rule profile.
func DealInitial(playerCount int, profile RuleProfile, opts DealOptions) (InitialDeal, error) {
	if err := validateDealInput(playerCount, profile); err != nil {
		return InitialDeal{}, err
	}

	shuffle := opts.Shuffle
	if shuffle == nil {
		shuffle = func([]Card) {}
	}

	maxAttempts := max(profile.MaxSetupAttempts, 1)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		deck := NewDeck36()
		shuffle(deck)

		hands, stock := dealHands(deck, playerCount, profile.InitialHandSize)
		if hasRedealHand(hands, profile.RedealSameSuitThreshold) {
			continue
		}

		trump, reselections, err := selectTrumpIndicator(stock, profile, shuffle)
		if err != nil {
			return InitialDeal{}, err
		}

		firstAttacker, random := selectFirstAttacker(hands, trump.Suit, opts.Choose)
		return InitialDeal{
			Hands:               cloneHands(hands),
			Stock:               slices.Clone(stock),
			TrumpIndicator:      trump,
			TrumpSuit:           trump.Suit,
			FirstAttacker:       firstAttacker,
			Redeals:             attempt,
			TrumpReselections:   reselections,
			RandomFirstAttacker: random,
		}, nil
	}

	return InitialDeal{}, ErrSetupAttemptsLimit
}

func validateDealInput(playerCount int, profile RuleProfile) error {
	if playerCount < 2 || playerCount > profile.MaxPlayers {
		return fmt.Errorf("%w: got %d, allowed 2..%d", ErrInvalidPlayerCount, playerCount, profile.MaxPlayers)
	}
	requiredCards := playerCount*profile.InitialHandSize + 1
	if requiredCards > len(NewDeck36()) {
		return fmt.Errorf("%w: %d players need %d cards including trump indicator", ErrInvalidPlayerCount, playerCount, requiredCards)
	}
	return nil
}

func dealHands(deck []Card, playerCount, handSize int) ([][]Card, []Card) {
	hands := make([][]Card, playerCount)
	for cardIndex := range handSize {
		for player := range playerCount {
			deckIndex := cardIndex*playerCount + player
			hands[player] = append(hands[player], deck[deckIndex])
		}
	}

	stockStart := playerCount * handSize
	return hands, slices.Clone(deck[stockStart:])
}

func hasRedealHand(hands [][]Card, threshold int) bool {
	for _, hand := range hands {
		counts := make(map[Suit]int, len(AllSuits()))
		for _, card := range hand {
			counts[card.Suit]++
			if counts[card.Suit] >= threshold {
				return true
			}
		}
	}
	return false
}

func selectTrumpIndicator(stock []Card, profile RuleProfile, shuffle ShuffleFunc) (Card, int, error) {
	maxAttempts := max(profile.MaxSetupAttempts, 1)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		trump := stock[len(stock)-1]
		if trump.Rank != profile.TrumpIndicatorForbiddenRank {
			return trump, attempt, nil
		}
		shuffle(stock)
	}
	return Card{}, 0, ErrSetupAttemptsLimit
}

func selectFirstAttacker(hands [][]Card, trumpSuit Suit, choose IntnFunc) (int, bool) {
	firstAttacker := -1
	var lowestTrump Rank

	for player, hand := range hands {
		for _, card := range hand {
			if card.Suit != trumpSuit {
				continue
			}
			if firstAttacker == -1 || card.Rank < lowestTrump {
				firstAttacker = player
				lowestTrump = card.Rank
			}
		}
	}

	if firstAttacker != -1 {
		return firstAttacker, false
	}
	if choose == nil {
		return 0, true
	}
	return choose(len(hands)), true
}

func cloneHands(hands [][]Card) [][]Card {
	cloned := make([][]Card, len(hands))
	for i, hand := range hands {
		cloned[i] = slices.Clone(hand)
	}
	return cloned
}
