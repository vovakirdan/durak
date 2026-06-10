package domain_test

import (
	"testing"

	. "github.com/vovakirdan/durak/internal/domain"
)

func TestCanBeat(t *testing.T) {
	tests := []struct {
		name    string
		attack  Card
		defense Card
		want    bool
	}{
		{
			name:    "higher card of same suit beats",
			attack:  Card{Rank: Ten, Suit: Clubs},
			defense: Card{Rank: Jack, Suit: Clubs},
			want:    true,
		},
		{
			name:    "lower card of same suit does not beat",
			attack:  Card{Rank: King, Suit: Clubs},
			defense: Card{Rank: Queen, Suit: Clubs},
			want:    false,
		},
		{
			name:    "trump beats non-trump",
			attack:  Card{Rank: Ace, Suit: Clubs},
			defense: Card{Rank: Six, Suit: Hearts},
			want:    true,
		},
		{
			name:    "non-trump does not beat different non-trump",
			attack:  Card{Rank: Six, Suit: Clubs},
			defense: Card{Rank: Ace, Suit: Diamonds},
			want:    false,
		},
		{
			name:    "non-trump does not beat trump",
			attack:  Card{Rank: Six, Suit: Hearts},
			defense: Card{Rank: Ace, Suit: Clubs},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanBeat(tt.attack, tt.defense, Hearts); got != tt.want {
				t.Fatalf("CanBeat(%v, %v, Hearts) = %t, want %t", tt.attack, tt.defense, got, tt.want)
			}
		})
	}
}
