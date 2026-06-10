package domain

import "fmt"

// Suit identifies a card suit.
type Suit uint8

const (
	// SuitUnknown is the zero value for an unset suit.
	SuitUnknown Suit = iota
	// Clubs is the clubs suit.
	Clubs
	// Diamonds is the diamonds suit.
	Diamonds
	// Hearts is the hearts suit.
	Hearts
	// Spades is the spades suit.
	Spades
)

// Rank identifies a card rank.
type Rank uint8

const (
	// RankUnknown is the zero value for an unset rank.
	RankUnknown Rank = iota
	// Six is the lowest rank in the 36-card Durak deck.
	Six Rank = 6
	// Seven is a Durak card rank.
	Seven Rank = 7
	// Eight is a Durak card rank.
	Eight Rank = 8
	// Nine is a Durak card rank.
	Nine Rank = 9
	// Ten is a Durak card rank.
	Ten Rank = 10
	// Jack is a Durak card rank.
	Jack Rank = 11
	// Queen is a Durak card rank.
	Queen Rank = 12
	// King is a Durak card rank.
	King Rank = 13
	// Ace is the highest rank in the 36-card Durak deck.
	Ace Rank = 14
)

// Card is a single playing card.
type Card struct {
	Rank Rank
	Suit Suit
}

// NewDeck36 returns a 36-card Durak deck ordered by suit and then rank.
func NewDeck36() []Card {
	deck := make([]Card, 0, 36)
	for _, suit := range AllSuits() {
		for _, rank := range DurakRanks() {
			deck = append(deck, Card{Rank: rank, Suit: suit})
		}
	}
	return deck
}

// AllSuits returns all suits in stable deck order.
func AllSuits() []Suit {
	return []Suit{Clubs, Diamonds, Hearts, Spades}
}

// DurakRanks returns ranks used in a 36-card Durak deck.
func DurakRanks() []Rank {
	return []Rank{Six, Seven, Eight, Nine, Ten, Jack, Queen, King, Ace}
}

func (s Suit) String() string {
	switch s {
	case Clubs:
		return "C"
	case Diamonds:
		return "D"
	case Hearts:
		return "H"
	case Spades:
		return "S"
	default:
		return "?"
	}
}

func (r Rank) String() string {
	switch r {
	case Six, Seven, Eight, Nine, Ten:
		return fmt.Sprintf("%d", r)
	case Jack:
		return "J"
	case Queen:
		return "Q"
	case King:
		return "K"
	case Ace:
		return "A"
	default:
		return "?"
	}
}

func (c Card) String() string {
	return c.Rank.String() + c.Suit.String()
}
