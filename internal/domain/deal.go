package domain

import (
	"errors"
	"fmt"
	rand "math/rand/v2"
	"slices"
)

var (
	// ErrInvalidPlayerCount means the requested player count cannot be dealt.
	ErrInvalidPlayerCount = errors.New("invalid player count")
	// ErrSetupAttemptsLimit means setup could not satisfy rules in time.
	ErrSetupAttemptsLimit = errors.New("setup attempts limit reached")
)

// Shuffler mutates cards into a shuffled order.
type Shuffler interface {
	Shuffle(cards []Card)
}

// ShuffleFunc adapts a function into a Shuffler.
type ShuffleFunc func(cards []Card)

// Shuffle mutates cards by calling fn.
func (fn ShuffleFunc) Shuffle(cards []Card) {
	fn(cards)
}

// IntnFunc returns an integer in [0, n).
type IntnFunc func(n int) int

// DealOptions contains injectable randomness for deterministic deals.
type DealOptions struct {
	Shuffler Shuffler
	Choose   IntnFunc
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

	shuffler := opts.Shuffler
	if shuffler == nil {
		shuffler = defaultShuffler{}
	}

	maxAttempts := max(profile.MaxSetupAttempts, 1)
	for attempt := range maxAttempts {
		deck := NewDeck36()
		shuffler.Shuffle(deck)

		hands, stock := dealHands(deck, playerCount, profile.InitialHandSize)
		if hasRedealHand(hands, profile.RedealSameSuitThreshold) {
			continue
		}

		reselections := 0
		trump := lastDealtCard(hands)
		if len(stock) > 0 {
			selected, selectedReshuffles, err := selectTrumpIndicator(stock, profile, shuffler)
			if err != nil {
				return InitialDeal{}, err
			}
			trump = selected
			reselections = selectedReshuffles
		}
		if trump.Rank == profile.TrumpIndicatorForbiddenRank {
			continue
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
	requiredCards := playerCount * profile.InitialHandSize
	if requiredCards > len(NewDeck36()) {
		return fmt.Errorf("%w: %d players need %d cards", ErrInvalidPlayerCount, playerCount, requiredCards)
	}
	return nil
}

type defaultShuffler struct{}

// Shuffle randomly reorders cards with the standard library RNG.
func (defaultShuffler) Shuffle(cards []Card) {
	rand.Shuffle(len(cards), func(i, j int) {
		cards[i], cards[j] = cards[j], cards[i]
	})
}

func dealHands(deck []Card, playerCount, handSize int) (hands [][]Card, stock []Card) {
	hands = make([][]Card, playerCount)
	for cardIndex := range handSize {
		for player := range playerCount {
			deckIndex := cardIndex*playerCount + player
			hands[player] = append(hands[player], deck[deckIndex])
		}
	}

	stockStart := playerCount * handSize
	return hands, slices.Clone(deck[stockStart:])
}

func lastDealtCard(hands [][]Card) Card {
	if len(hands) == 0 {
		return Card{}
	}
	lastHand := hands[len(hands)-1]
	if len(lastHand) == 0 {
		return Card{}
	}
	return lastHand[len(lastHand)-1]
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

func selectTrumpIndicator(stock []Card, profile RuleProfile, shuffler Shuffler) (Card, int, error) {
	maxAttempts := max(profile.MaxSetupAttempts, 1)
	for attempt := range maxAttempts {
		trump := stock[len(stock)-1]
		if trump.Rank != profile.TrumpIndicatorForbiddenRank {
			return trump, attempt, nil
		}
		shuffler.Shuffle(stock)
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
