package domain

import "slices"

// EventKind identifies a domain event emitted by match state transitions.
type EventKind uint8

const (
	// EventKindMatchStarted marks a new in-memory match.
	EventKindMatchStarted EventKind = iota + 1
	// EventKindDeal records public initial deal metadata.
	EventKindDeal
	// EventKindAttack records an accepted attack action.
	EventKindAttack
	// EventKindDefend records an accepted defense action.
	EventKindDefend
	// EventKindThrowIn records an accepted throw-in action.
	EventKindThrowIn
	// EventKindTransfer records an accepted transfer action.
	EventKindTransfer
	// EventKindTake records a defender choosing to take cards.
	EventKindTake
	// EventKindFinishDefense records a successful defense completion.
	EventKindFinishDefense
	// EventKindFinishTake records a take completion.
	EventKindFinishTake
	// EventKindRefill records public refill counts after a round.
	EventKindRefill
	// EventKindRoundEnded records the resolved round outcome.
	EventKindRoundEnded
	// EventKindConcede records a seat conceding the match.
	EventKindConcede
	// EventKindMatchEnded records the completed match outcome.
	EventKindMatchEnded
)

// RoundOutcome identifies how a round resolved.
type RoundOutcome uint8

const (
	// RoundOutcomeUnknown is the zero value for an unset outcome.
	RoundOutcomeUnknown RoundOutcome = iota
	// RoundOutcomeDefense means all attacks were beaten.
	RoundOutcomeDefense
	// RoundOutcomeTake means the defender picked up table cards.
	RoundOutcomeTake
)

// Event is a structured domain event with one payload matching Kind.
type Event struct {
	Kind       EventKind
	Started    *MatchStartedEvent
	Deal       *DealEvent
	Action     *ActionEvent
	Refill     *RefillEvent
	RoundEnded *RoundEndedEvent
	Concede    *ConcedeEvent
	MatchEnded *MatchEndedEvent
}

// MatchStartedEvent describes match creation without persistence identifiers.
type MatchStartedEvent struct {
	PlayerCount int
	RuleProfile string
}

// DealEvent records public setup data and omits hidden hand contents.
type DealEvent struct {
	TrumpIndicator      Card
	TrumpSuit           Suit
	FirstAttacker       Seat
	Defender            Seat
	HandSizes           []int
	StockCount          int
	Redeals             int
	TrumpReselections   int
	RandomFirstAttacker bool
}

// ActionEvent records an accepted domain action.
type ActionEvent struct {
	Action Action
}

// RefillEvent records public card counts after drawing from stock.
type RefillEvent struct {
	Seat       Seat
	Drawn      int
	HandSize   int
	StockCount int
}

// RoundEndedEvent records public round resolution and next roles.
type RoundEndedEvent struct {
	Outcome            RoundOutcome
	Attacker           Seat
	Defender           Seat
	Cards              []Card
	NextAttacker       Seat
	NextDefender       Seat
	SuccessfulDefenses int
}

// ConcedeEvent records a non-rule user concession.
type ConcedeEvent struct {
	Seat   Seat
	Winner Seat
}

// MatchEndedEvent records final match outcome.
type MatchEndedEvent struct {
	Winner Seat
	Loser  Seat
	Draw   bool
}

// Clone returns a deep copy of event payload slices and pointers.
func (e Event) Clone() Event {
	if e.Started != nil {
		started := *e.Started
		e.Started = &started
	}
	if e.Deal != nil {
		deal := *e.Deal
		deal.HandSizes = slices.Clone(deal.HandSizes)
		e.Deal = &deal
	}
	if e.Action != nil {
		action := *e.Action
		e.Action = &action
	}
	if e.Refill != nil {
		refill := *e.Refill
		e.Refill = &refill
	}
	if e.RoundEnded != nil {
		roundEnded := *e.RoundEnded
		roundEnded.Cards = slices.Clone(roundEnded.Cards)
		e.RoundEnded = &roundEnded
	}
	if e.Concede != nil {
		concede := *e.Concede
		e.Concede = &concede
	}
	if e.MatchEnded != nil {
		matchEnded := *e.MatchEnded
		e.MatchEnded = &matchEnded
	}
	return e
}

func cloneEvents(events []Event) []Event {
	cloned := make([]Event, len(events))
	for i, event := range events {
		cloned[i] = event.Clone()
	}
	return cloned
}

func (m *Match) appendMatchStartedEvent(profile RuleProfile) {
	m.appendEvent(Event{
		Kind: EventKindMatchStarted,
		Started: &MatchStartedEvent{
			PlayerCount: len(m.hands),
			RuleProfile: profile.Name,
		},
	})
}

func (m *Match) appendDealEvent(deal *InitialDeal) {
	m.appendEvent(Event{
		Kind: EventKindDeal,
		Deal: &DealEvent{
			TrumpIndicator:      deal.TrumpIndicator,
			TrumpSuit:           deal.TrumpSuit,
			FirstAttacker:       Seat(deal.FirstAttacker),
			Defender:            m.defender,
			HandSizes:           m.handSizes(),
			StockCount:          len(m.stock),
			Redeals:             deal.Redeals,
			TrumpReselections:   deal.TrumpReselections,
			RandomFirstAttacker: deal.RandomFirstAttacker,
		},
	})
}

func (m *Match) appendActionEvent(kind EventKind, action Action) {
	m.appendEvent(Event{
		Kind:   kind,
		Action: &ActionEvent{Action: action},
	})
}

func (m *Match) appendRoundEndedEvent(outcome RoundOutcome, attacker, defender Seat, cards []Card) {
	m.appendEvent(Event{
		Kind: EventKindRoundEnded,
		RoundEnded: &RoundEndedEvent{
			Outcome:            outcome,
			Attacker:           attacker,
			Defender:           defender,
			Cards:              slices.Clone(cards),
			NextAttacker:       m.attacker,
			NextDefender:       m.defender,
			SuccessfulDefenses: m.successfulDefenses,
		},
	})
}

func (m *Match) appendMatchEndedEvent() {
	m.appendEvent(Event{
		Kind: EventKindMatchEnded,
		MatchEnded: &MatchEndedEvent{
			Winner: m.winner,
			Loser:  m.loser,
			Draw:   m.winner == NoSeat,
		},
	})
}

func (m *Match) appendEvent(event Event) {
	m.events = append(m.events, event)
}

func (m *Match) handSizes() []int {
	sizes := make([]int, len(m.hands))
	for seat, hand := range m.hands {
		sizes[seat] = len(hand)
	}
	return sizes
}
