package evaluation_test

import (
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestBuildHiddenCardsStartPosition(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			TrumpIndicator: card(domain.Ace, domain.Hearts),
		},
		Hand: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Seven, domain.Clubs),
			card(domain.Eight, domain.Clubs),
			card(domain.Nine, domain.Clubs),
			card(domain.Ten, domain.Clubs),
			card(domain.Jack, domain.Clubs),
		},
	}

	hidden := evaluation.BuildHiddenCards(&decision, nil)

	if len(hidden.Known) != 7 {
		t.Fatalf("known length = %d, want 7", len(hidden.Known))
	}
	if len(hidden.UnknownPool) != 29 {
		t.Fatalf("unknown pool length = %d, want 29", len(hidden.UnknownPool))
	}
	if got := hidden.OpponentCardProbability(card(domain.Ace, domain.Spades), 6); got <= 0.20 || got >= 0.21 {
		t.Fatalf("opponent probability = %.3f, want about 6/29", got)
	}
}

func TestBuildHiddenCardsLateGameCollapsesOpponentRemainder(t *testing.T) {
	deck := domain.NewDeck36()
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			TrumpIndicator: deck[4],
			Table: []domain.TablePair{
				{Attack: deck[2], Defense: deck[3], Defended: true},
			},
		},
		Hand: deck[:2],
	}

	hidden := evaluation.BuildHiddenCards(&decision, deck[5:34])

	if len(hidden.UnknownPool) != 2 {
		t.Fatalf("unknown pool length = %d, want 2", len(hidden.UnknownPool))
	}
	for _, card := range deck[34:36] {
		if !hidden.IsUnknown(card) {
			t.Fatalf("%v is not unknown, want possible opponent card", card)
		}
		if got := hidden.OpponentCardProbability(card, 2); got != 1 {
			t.Fatalf("opponent probability for %v = %.3f, want 1", card, got)
		}
	}
	if got := hidden.OpponentCardProbability(deck[0], 2); got != 0 {
		t.Fatalf("known card probability = %.3f, want 0", got)
	}
}

func TestBuildHiddenCardsKeepsKnownOpponentCards(t *testing.T) {
	knownOpponent := card(domain.Ace, domain.Spades)
	own := card(domain.Six, domain.Clubs)
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:           domain.Seat(0),
			TrumpIndicator: card(domain.Nine, domain.Hearts),
			HandSizes:      []int{1, 1},
		},
		Hand: []domain.Card{own},
		PublicMemory: app.PublicCardMemory{
			Seat:           domain.Seat(0),
			Hand:           []domain.Card{own},
			TrumpIndicator: card(domain.Nine, domain.Hearts),
			KnownHeld: [][]domain.Card{
				{own},
				{knownOpponent},
			},
			Seen: []domain.Card{
				own,
				knownOpponent,
				card(domain.Nine, domain.Hearts),
			},
		},
	}

	hidden := evaluation.BuildHiddenCards(&decision, nil)

	if !hidden.KnownByOpponent(knownOpponent) {
		t.Fatalf("%v is not known by opponent", knownOpponent)
	}
	if got := hidden.OpponentCardProbability(knownOpponent, 1); got != 1 {
		t.Fatalf("known opponent probability = %.3f, want 1", got)
	}
	if slices.Contains(hidden.UnknownPool, knownOpponent) {
		t.Fatalf("%v in unknown pool, want known opponent card removed", knownOpponent)
	}
	if got := hidden.OpponentCardProbability(own, 1); got != 0 {
		t.Fatalf("own card probability = %.3f, want 0", got)
	}
}

func TestBuildHiddenCardsUsesPublicMemoryDiscard(t *testing.T) {
	own := card(domain.Six, domain.Clubs)
	discarded := card(domain.Ace, domain.Spades)
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:      domain.Seat(0),
			TrumpSuit: domain.Hearts,
			HandSizes: []int{1, 1},
		},
		Hand: []domain.Card{own},
		PublicMemory: app.PublicCardMemory{
			Seat:    domain.Seat(0),
			Hand:    []domain.Card{own},
			Discard: []domain.Card{discarded},
			Seen:    []domain.Card{own},
		},
	}

	hidden := evaluation.BuildHiddenCards(&decision, nil)

	if !hidden.IsKnown(discarded) {
		t.Fatalf("%v is not known, want public discard included", discarded)
	}
	if slices.Contains(hidden.UnknownPool, discarded) {
		t.Fatalf("%v in unknown pool, want public discard removed", discarded)
	}
}
