package domain

import "fmt"

// Suit identifies a card suit.
type Suit uint8

const (
	SuitUnknown Suit = iota
	Clubs
	Diamonds
	Hearts
	Spades
)

// Rank identifies a card rank.
type Rank uint8

const (
	RankUnknown Rank = iota
	Six         Rank = 6
	Seven       Rank = 7
	Eight       Rank = 8
	Nine        Rank = 9
	Ten         Rank = 10
	Jack        Rank = 11
	Queen       Rank = 12
	King        Rank = 13
	Ace         Rank = 14
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
