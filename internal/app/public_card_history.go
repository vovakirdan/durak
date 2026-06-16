package app

import (
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

// PublicCardMemory is a seat-scoped runtime snapshot of cards a real player
// could remember or infer from visible play.
type PublicCardMemory struct {
	Seat           domain.Seat
	Hand           []domain.Card
	Table          []domain.TablePair
	Discard        []domain.Card
	KnownHeld      [][]domain.Card
	Seen           []domain.Card
	UnknownPool    []domain.Card
	RankSeen       map[domain.Rank]int
	SuitSeen       map[domain.Suit]int
	HandSizes      []int
	StockCount     int
	TrumpIndicator domain.Card
	TrumpSuit      domain.Suit
	Confidence     int
}

// Clone returns a deep copy of the memory snapshot.
func (m *PublicCardMemory) Clone() PublicCardMemory {
	if m == nil {
		return PublicCardMemory{}
	}
	return PublicCardMemory{
		Seat:           m.Seat,
		Hand:           slices.Clone(m.Hand),
		Table:          slices.Clone(m.Table),
		Discard:        slices.Clone(m.Discard),
		KnownHeld:      cloneCardGroups(m.KnownHeld),
		Seen:           slices.Clone(m.Seen),
		UnknownPool:    slices.Clone(m.UnknownPool),
		RankSeen:       cloneRankCounts(m.RankSeen),
		SuitSeen:       cloneSuitCounts(m.SuitSeen),
		HandSizes:      slices.Clone(m.HandSizes),
		StockCount:     m.StockCount,
		TrumpIndicator: m.TrumpIndicator,
		TrumpSuit:      m.TrumpSuit,
		Confidence:     m.Confidence,
	}
}

// PublicCardHistory updates public card memory from visible match events.
type PublicCardHistory struct {
	playerCount    int
	handSizes      []int
	stockCount     int
	trumpIndicator domain.Card
	trumpSuit      domain.Suit
	discard        []domain.Card
	knownHeld      [][]domain.Card
	seen           []domain.Card
}

// NewPublicCardHistory creates an empty runtime public-card tracker.
func NewPublicCardHistory() PublicCardHistory {
	return PublicCardHistory{}
}

// Apply records one visible domain event into runtime memory.
func (h *PublicCardHistory) Apply(event domain.Event) {
	if h == nil {
		return
	}
	switch event.Kind {
	case domain.EventKindMatchStarted:
		if event.Started != nil {
			h.ensureSeats(event.Started.PlayerCount)
		}
	case domain.EventKindDeal:
		h.applyDeal(event.Deal)
	case domain.EventKindAttack, domain.EventKindDefend, domain.EventKindThrowIn,
		domain.EventKindTransfer:
		h.applyCardAction(event.Action)
	case domain.EventKindRefill:
		h.applyRefill(event.Refill)
	case domain.EventKindRoundEnded:
		h.applyRoundEnded(event.RoundEnded)
	}
}

// Snapshot builds a seat-view memory snapshot and overlays the current private
// hand from the decision context.
func (h *PublicCardHistory) Snapshot(seat domain.Seat, decision *DecisionContext) PublicCardMemory {
	if h == nil {
		return PublicCardMemory{}
	}
	memory := PublicCardMemory{
		Seat:           seat,
		Discard:        slices.Clone(h.discard),
		KnownHeld:      cloneCardGroups(h.knownHeld),
		Seen:           slices.Clone(h.seen),
		HandSizes:      slices.Clone(h.handSizes),
		StockCount:     h.stockCount,
		TrumpIndicator: h.trumpIndicator,
		TrumpSuit:      h.trumpSuit,
	}
	if decision != nil {
		memory.Seat = decision.Seat
		memory.Hand = slices.Clone(decision.Hand)
		memory.Table = slices.Clone(decision.Table)
		memory.HandSizes = slices.Clone(decision.HandSizes)
		memory.StockCount = decision.StockCount
		if memory.TrumpIndicator == (domain.Card{}) {
			memory.TrumpIndicator = decision.TrumpIndicator
		}
		if memory.TrumpSuit == domain.SuitUnknown {
			memory.TrumpSuit = decision.TrumpSuit
		}
		memory.ensureKnownHeldSeat(decision.Seat)
		if memory.validKnownHeldSeat(decision.Seat) {
			memory.KnownHeld[int(decision.Seat)] = slices.Clone(decision.Hand)
		}
	}
	memory.Seen = buildSeenCards(&memory)
	memory.UnknownPool = buildUnknownPool(memory.Seen)
	memory.RankSeen = rankCounts(memory.Seen)
	memory.SuitSeen = suitCounts(memory.Seen)
	memory.Confidence = publicMemoryConfidence(len(memory.UnknownPool))
	return memory
}

func (h *PublicCardHistory) applyDeal(deal *domain.DealEvent) {
	if deal == nil {
		return
	}
	h.ensureSeats(len(deal.HandSizes))
	h.handSizes = slices.Clone(deal.HandSizes)
	h.stockCount = deal.StockCount
	h.trumpIndicator = deal.TrumpIndicator
	h.trumpSuit = deal.TrumpSuit
	h.seen = appendUniqueCards(h.seen, deal.TrumpIndicator)
}

func (h *PublicCardHistory) applyCardAction(event *domain.ActionEvent) {
	if event == nil || !validPublicCard(event.Action.Card) {
		return
	}
	card := event.Action.Card
	h.seen = appendUniqueCards(h.seen, card)
	h.removeKnownHeld(event.Action.Seat, card)
}

func (h *PublicCardHistory) applyRefill(refill *domain.RefillEvent) {
	if refill == nil {
		return
	}
	h.ensureSeats(int(refill.Seat) + 1)
	if validPublicSeat(refill.Seat, len(h.handSizes)) {
		h.handSizes[int(refill.Seat)] = refill.HandSize
	}
	h.stockCount = refill.StockCount
}

func (h *PublicCardHistory) applyRoundEnded(event *domain.RoundEndedEvent) {
	if event == nil {
		return
	}
	h.seen = appendUniqueCards(h.seen, event.Cards...)
	switch event.Outcome {
	case domain.RoundOutcomeDefense:
		h.discard = appendUniqueCards(h.discard, event.Cards...)
	case domain.RoundOutcomeTake:
		h.ensureSeats(int(event.Defender) + 1)
		if validPublicSeat(event.Defender, len(h.knownHeld)) {
			h.knownHeld[int(event.Defender)] = appendUniqueCards(h.knownHeld[int(event.Defender)], event.Cards...)
		}
	}
}

func (h *PublicCardHistory) ensureSeats(count int) {
	if count <= h.playerCount {
		return
	}
	h.playerCount = count
	if len(h.handSizes) < count {
		h.handSizes = append(h.handSizes, make([]int, count-len(h.handSizes))...)
	}
	if len(h.knownHeld) < count {
		h.knownHeld = append(h.knownHeld, make([][]domain.Card, count-len(h.knownHeld))...)
	}
}

func (h *PublicCardHistory) removeKnownHeld(seat domain.Seat, card domain.Card) {
	if !validPublicSeat(seat, len(h.knownHeld)) {
		return
	}
	h.knownHeld[int(seat)] = removeCard(h.knownHeld[int(seat)], card)
}

func (m *PublicCardMemory) ensureKnownHeldSeat(seat domain.Seat) {
	if seat == domain.NoSeat || int(seat) < len(m.KnownHeld) {
		return
	}
	m.KnownHeld = append(m.KnownHeld, make([][]domain.Card, int(seat)-len(m.KnownHeld)+1)...)
}

func (m *PublicCardMemory) validKnownHeldSeat(seat domain.Seat) bool {
	return validPublicSeat(seat, len(m.KnownHeld))
}

func buildSeenCards(memory *PublicCardMemory) []domain.Card {
	if memory == nil {
		return nil
	}
	seen := appendUniqueCards(nil, memory.Seen...)
	seen = appendUniqueCards(seen, memory.TrumpIndicator)
	seen = appendUniqueCards(seen, memory.Hand...)
	for _, pair := range memory.Table {
		seen = appendUniqueCards(seen, pair.Attack)
		if pair.Defended {
			seen = appendUniqueCards(seen, pair.Defense)
		}
	}
	seen = appendUniqueCards(seen, memory.Discard...)
	for _, cards := range memory.KnownHeld {
		seen = appendUniqueCards(seen, cards...)
	}
	return seen
}

func buildUnknownPool(known []domain.Card) []domain.Card {
	knownSet := make(map[domain.Card]bool, len(known))
	for _, card := range known {
		if validPublicCard(card) {
			knownSet[card] = true
		}
	}
	unknown := make([]domain.Card, 0, len(domain.NewDeck36())-len(knownSet))
	for _, card := range domain.NewDeck36() {
		if !knownSet[card] {
			unknown = append(unknown, card)
		}
	}
	return unknown
}

func appendUniqueCards(cards []domain.Card, candidates ...domain.Card) []domain.Card {
	for _, card := range candidates {
		if !validPublicCard(card) || slices.Contains(cards, card) {
			continue
		}
		cards = append(cards, card)
	}
	return cards
}

func removeCard(cards []domain.Card, card domain.Card) []domain.Card {
	index := slices.Index(cards, card)
	if index < 0 {
		return cards
	}
	return slices.Delete(cards, index, index+1)
}

func rankCounts(cards []domain.Card) map[domain.Rank]int {
	counts := make(map[domain.Rank]int, len(cards))
	for _, card := range cards {
		if validPublicCard(card) {
			counts[card.Rank]++
		}
	}
	return counts
}

func suitCounts(cards []domain.Card) map[domain.Suit]int {
	counts := make(map[domain.Suit]int, len(cards))
	for _, card := range cards {
		if validPublicCard(card) {
			counts[card.Suit]++
		}
	}
	return counts
}

func publicMemoryConfidence(unknownCount int) int {
	confidence := 100 - unknownCount*2
	if unknownCount > 0 && confidence < 20 {
		confidence = 20
	}
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return confidence
}

func validPublicSeat(seat domain.Seat, seats int) bool {
	return seat != domain.NoSeat && int(seat) >= 0 && int(seat) < seats
}

func validPublicCard(card domain.Card) bool {
	return card.Rank != domain.RankUnknown && card.Suit != domain.SuitUnknown
}

func cloneCardGroups(groups [][]domain.Card) [][]domain.Card {
	cloned := make([][]domain.Card, len(groups))
	for i, group := range groups {
		cloned[i] = slices.Clone(group)
	}
	return cloned
}

func cloneRankCounts(counts map[domain.Rank]int) map[domain.Rank]int {
	if counts == nil {
		return nil
	}
	cloned := make(map[domain.Rank]int, len(counts))
	for rank, count := range counts {
		cloned[rank] = count
	}
	return cloned
}

func cloneSuitCounts(counts map[domain.Suit]int) map[domain.Suit]int {
	if counts == nil {
		return nil
	}
	cloned := make(map[domain.Suit]int, len(counts))
	for suit, count := range counts {
		cloned[suit] = count
	}
	return cloned
}
