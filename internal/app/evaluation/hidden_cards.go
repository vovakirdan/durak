package evaluation

import (
	"slices"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// HiddenCards is the seat-view model of known cards and the remaining unknown pool.
type HiddenCards struct {
	Seat        domain.Seat
	Known       []domain.Card
	KnownHeld   *[][]domain.Card
	UnknownPool []domain.Card
}

// BuildHiddenCards derives the unknown pool from public seat-view data.
func BuildHiddenCards(decision *app.DecisionContext, discard []domain.Card) HiddenCards {
	if decision == nil {
		return NewHiddenCards(discard)
	}
	if hasPublicMemory(&decision.PublicMemory) {
		known := appendKnownCards(nil, decision.PublicMemory.Seen...)
		known = appendKnownCards(known, decision.PublicMemory.Discard...)
		known = appendKnownCards(known, discard...)
		hidden := NewHiddenCards(known)
		hidden.Seat = decision.Seat
		hidden.KnownHeld = knownHeldPointer(decision.PublicMemory.KnownHeld)
		return hidden
	}
	known := make([]domain.Card, 0, len(decision.Hand)+len(decision.Table)*2+len(discard)+1)
	known = appendKnownCards(known, decision.Hand...)
	for _, pair := range decision.Table {
		known = appendKnownCards(known, pair.Attack)
		if pair.Defended {
			known = appendKnownCards(known, pair.Defense)
		}
	}
	known = appendKnownCards(known, decision.TrumpIndicator)
	known = appendKnownCards(known, discard...)
	hidden := NewHiddenCards(known)
	hidden.Seat = decision.Seat
	return hidden
}

// NewHiddenCards builds a model from already known cards.
func NewHiddenCards(known []domain.Card) HiddenCards {
	deduped := appendKnownCards(nil, known...)
	knownSet := cardSet(deduped)
	unknown := make([]domain.Card, 0, len(domain.NewDeck36())-len(knownSet))
	for _, card := range domain.NewDeck36() {
		if !knownSet[card] {
			unknown = append(unknown, card)
		}
	}
	return HiddenCards{
		Known:       deduped,
		UnknownPool: unknown,
	}
}

// IsKnown reports whether a card has a known location from the evaluated view.
func (h HiddenCards) IsKnown(card domain.Card) bool {
	return slices.Contains(h.Known, card)
}

// IsUnknown reports whether a card can still be in an opponent hand or stock.
func (h HiddenCards) IsUnknown(card domain.Card) bool {
	return slices.Contains(h.UnknownPool, card)
}

// OpponentCardProbability estimates whether one unknown card sits in one opponent hand.
func (h HiddenCards) OpponentCardProbability(card domain.Card, opponentHandSize int) float64 {
	if h.KnownByOpponent(card) {
		return 1
	}
	if opponentHandSize <= 0 || !h.IsUnknown(card) || len(h.UnknownPool) == 0 {
		return 0
	}
	probability := float64(opponentHandSize) / float64(len(h.UnknownPool))
	if probability > 1 {
		return 1
	}
	return probability
}

// KnownByOpponent reports whether a card is known to sit in another seat's hand.
func (h HiddenCards) KnownByOpponent(card domain.Card) bool {
	for seat, cards := range h.knownHeldGroups() {
		if domain.Seat(seat) == h.Seat {
			continue
		}
		if slices.Contains(cards, card) {
			return true
		}
	}
	return false
}

// KnownBySeat reports whether a card is known to sit in the given seat's hand.
func (h HiddenCards) KnownBySeat(seat domain.Seat, card domain.Card) bool {
	groups := h.knownHeldGroups()
	if seat == domain.NoSeat || int(seat) < 0 || int(seat) >= len(groups) {
		return false
	}
	return slices.Contains(groups[int(seat)], card)
}

func (h HiddenCards) knownHeldGroups() [][]domain.Card {
	if h.KnownHeld == nil {
		return nil
	}
	return *h.KnownHeld
}

func hasPublicMemory(memory *app.PublicCardMemory) bool {
	return memory != nil &&
		(len(memory.Seen) > 0 ||
			len(memory.KnownHeld) > 0 ||
			len(memory.Discard) > 0 ||
			len(memory.Hand) > 0 ||
			len(memory.Table) > 0)
}

func knownHeldPointer(groups [][]domain.Card) *[][]domain.Card {
	cloned := make([][]domain.Card, len(groups))
	for i, group := range groups {
		cloned[i] = slices.Clone(group)
	}
	return &cloned
}

func appendKnownCards(known []domain.Card, cards ...domain.Card) []domain.Card {
	for _, card := range cards {
		if !validCard(card) || slices.Contains(known, card) {
			continue
		}
		known = append(known, card)
	}
	return known
}

func cardSet(cards []domain.Card) map[domain.Card]bool {
	set := make(map[domain.Card]bool, len(cards))
	for _, card := range cards {
		set[card] = true
	}
	return set
}

func validCard(card domain.Card) bool {
	return card.Rank != domain.RankUnknown && card.Suit != domain.SuitUnknown
}
