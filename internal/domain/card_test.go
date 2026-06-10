package domain_test

import (
	"testing"

	. "github.com/vovakirdan/durak/internal/domain"
)

func TestNewDeck36(t *testing.T) {
	deck := NewDeck36()
	if len(deck) != 36 {
		t.Fatalf("deck length = %d, want 36", len(deck))
	}

	seen := make(map[Card]bool, len(deck))
	for _, card := range deck {
		if card.Rank < Six || card.Rank > Ace {
			t.Fatalf("unexpected rank in deck: %v", card)
		}
		if card.Suit < Clubs || card.Suit > Spades {
			t.Fatalf("unexpected suit in deck: %v", card)
		}
		if seen[card] {
			t.Fatalf("duplicate card in deck: %v", card)
		}
		seen[card] = true
	}
}

func TestCardString(t *testing.T) {
	tests := map[string]struct {
		card Card
		want string
	}{
		"six clubs":   {card: Card{Rank: Six, Suit: Clubs}, want: "6C"},
		"ace hearts":  {card: Card{Rank: Ace, Suit: Hearts}, want: "AH"},
		"queen spade": {card: Card{Rank: Queen, Suit: Spades}, want: "QS"},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := tt.card.String(); got != tt.want {
				t.Fatalf("Card.String() = %q, want %q", got, tt.want)
			}
		})
	}
}
