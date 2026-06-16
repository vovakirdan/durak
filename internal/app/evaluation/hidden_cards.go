package evaluation

import (
	"slices"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// HiddenCards is the seat-view model of known cards and the remaining unknown pool.
type HiddenCards struct {
	Known       []domain.Card
	UnknownPool []domain.Card
}

// BuildHiddenCards derives the unknown pool from public seat-view data.
func BuildHiddenCards(decision *app.DecisionContext, discard []domain.Card) HiddenCards {
	if decision == nil {
		return NewHiddenCards(discard)
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
	return NewHiddenCards(known)
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

// IsKnown reports whether a card is impossible for opponents because it is visible.
func (h HiddenCards) IsKnown(card domain.Card) bool {
	return slices.Contains(h.Known, card)
}

// IsUnknown reports whether a card can still be in an opponent hand or stock.
func (h HiddenCards) IsUnknown(card domain.Card) bool {
	return slices.Contains(h.UnknownPool, card)
}

// OpponentCardProbability estimates whether one unknown card sits in one opponent hand.
func (h HiddenCards) OpponentCardProbability(card domain.Card, opponentHandSize int) float64 {
	if opponentHandSize <= 0 || !h.IsUnknown(card) || len(h.UnknownPool) == 0 {
		return 0
	}
	probability := float64(opponentHandSize) / float64(len(h.UnknownPool))
	if probability > 1 {
		return 1
	}
	return probability
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
