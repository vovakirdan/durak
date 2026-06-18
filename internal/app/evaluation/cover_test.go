package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSinglePendingCoverProbabilityUsesKnownHeld(t *testing.T) {
	attack := card(domain.Ace, domain.Clubs)
	known := card(domain.Six, domain.Hearts)
	knownHeld := [][]domain.Card{{}, {known}}
	hidden := evaluation.HiddenCards{
		Seat:        domain.Seat(0),
		KnownHeld:   &knownHeld,
		UnknownPool: domain.NewDeck36(),
	}

	got := evaluation.CoverProbability([]domain.Card{attack}, domain.Seat(1), 1, domain.Hearts, hidden)

	if got != 1 {
		t.Fatalf("probability = %v, want known cover", got)
	}
}

func TestSinglePendingCoverProbabilityReturnsZeroWithoutBeaters(t *testing.T) {
	attack := card(domain.Ace, domain.Hearts)
	hidden := evaluation.HiddenCards{
		Seat: domain.Seat(0),
		UnknownPool: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Seven, domain.Clubs),
		},
	}

	got := evaluation.CoverProbability([]domain.Card{attack}, domain.Seat(1), 2, domain.Hearts, hidden)

	if got != 0 {
		t.Fatalf("probability = %v, want no cover", got)
	}
}

func TestCanCoverAllRequiresDistinctDefenseCards(t *testing.T) {
	pending := []domain.Card{card(domain.Six, domain.Clubs), card(domain.Seven, domain.Clubs)}
	hand := []domain.Card{card(domain.Ace, domain.Clubs)}

	if evaluation.CanCoverAll(pending, hand, domain.Hearts) {
		t.Fatalf("one card must not cover two attacks")
	}
}

func TestCanCoverAllFindsMatchingAcrossSuits(t *testing.T) {
	pending := []domain.Card{card(domain.Six, domain.Clubs), card(domain.Seven, domain.Diamonds)}
	hand := []domain.Card{card(domain.Seven, domain.Clubs), card(domain.Eight, domain.Diamonds)}

	if !evaluation.CanCoverAll(pending, hand, domain.Hearts) {
		t.Fatalf("expected matching cover")
	}
}
