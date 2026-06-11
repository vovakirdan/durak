package app

import (
	"context"
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

// MatchID identifies one match event stream.
type MatchID string

// Event is an application-level sequenced domain event.
type Event struct {
	MatchID  MatchID
	Sequence uint64
	Domain   domain.Event
}

// InternalDealEvent records full setup state needed for exact replay/resume.
type InternalDealEvent struct {
	Hands               [][]domain.Card
	Stock               []domain.Card
	TrumpIndicator      domain.Card
	TrumpSuit           domain.Suit
	FirstAttacker       domain.Seat
	Defender            domain.Seat
	Redeals             int
	TrumpReselections   int
	RandomFirstAttacker bool
}

// InternalEvent is a sequenced canonical event for exact match replay.
type InternalEvent struct {
	MatchID  MatchID
	Sequence uint64
	Domain   domain.Event
	Deal     *InternalDealEvent
}

// EventStore appends sequenced events emitted by active sessions.
type EventStore interface {
	AppendEvents(context.Context, []Event) error
}

// InternalEventStore appends canonical events that may include hidden state.
type InternalEventStore interface {
	AppendInternalEvents(context.Context, []InternalEvent) error
}

// InMemoryEventStore stores events for tests and future local history wiring.
type InMemoryEventStore struct {
	events []Event
}

// InMemoryInternalEventStore stores canonical events for tests.
type InMemoryInternalEventStore struct {
	events []InternalEvent
}

// NewInMemoryEventStore creates an empty event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{}
}

// NewInMemoryInternalEventStore creates an empty internal event store.
func NewInMemoryInternalEventStore() *InMemoryInternalEventStore {
	return &InMemoryInternalEventStore{}
}

// AppendEvents stores events unless context has already been canceled.
func (s *InMemoryEventStore) AppendEvents(ctx context.Context, events []Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.events = append(s.events, cloneEvents(events)...)
	return nil
}

// Events returns copied stored events.
func (s *InMemoryEventStore) Events() []Event {
	return cloneEvents(s.events)
}

// AppendInternalEvents stores canonical events unless context has been canceled.
func (s *InMemoryInternalEventStore) AppendInternalEvents(ctx context.Context, events []InternalEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.events = append(s.events, cloneInternalEvents(events)...)
	return nil
}

// Events returns copied stored internal events.
func (s *InMemoryInternalEventStore) Events() []InternalEvent {
	return cloneInternalEvents(s.events)
}

// EventsForMatch returns copied internal events for one match stream.
func (s *InMemoryInternalEventStore) EventsForMatch(matchID MatchID) []InternalEvent {
	events := make([]InternalEvent, 0, len(s.events))
	for i := range s.events {
		if s.events[i].MatchID == matchID {
			events = append(events, cloneInternalEvent(&s.events[i]))
		}
	}
	return events
}

// EventsForMatch returns copied events for one match stream.
func (s *InMemoryEventStore) EventsForMatch(matchID MatchID) []Event {
	events := make([]Event, 0, len(s.events))
	for i := range s.events {
		if s.events[i].MatchID == matchID {
			events = append(events, cloneEvent(&s.events[i]))
		}
	}
	return events
}

// NewInternalDealEvent converts a domain setup deal into canonical replay data.
func NewInternalDealEvent(deal *domain.InitialDeal, defender domain.Seat) InternalDealEvent {
	if deal == nil {
		return InternalDealEvent{}
	}
	return InternalDealEvent{
		Hands:               cloneHands(deal.Hands),
		Stock:               slices.Clone(deal.Stock),
		TrumpIndicator:      deal.TrumpIndicator,
		TrumpSuit:           deal.TrumpSuit,
		FirstAttacker:       domain.Seat(deal.FirstAttacker),
		Defender:            defender,
		Redeals:             deal.Redeals,
		TrumpReselections:   deal.TrumpReselections,
		RandomFirstAttacker: deal.RandomFirstAttacker,
	}
}

// PublicDeal derives the safe public setup event from a canonical deal event.
func (e *InternalDealEvent) PublicDeal() domain.DealEvent {
	if e == nil {
		return domain.DealEvent{}
	}
	handSizes := make([]int, len(e.Hands))
	for seat, hand := range e.Hands {
		handSizes[seat] = len(hand)
	}
	return domain.DealEvent{
		TrumpIndicator:      e.TrumpIndicator,
		TrumpSuit:           e.TrumpSuit,
		FirstAttacker:       e.FirstAttacker,
		Defender:            e.Defender,
		HandSizes:           handSizes,
		StockCount:          len(e.Stock),
		Redeals:             e.Redeals,
		TrumpReselections:   e.TrumpReselections,
		RandomFirstAttacker: e.RandomFirstAttacker,
	}
}

// InitialDeal derives the full domain setup state from a canonical deal event.
func (e *InternalDealEvent) InitialDeal() domain.InitialDeal {
	if e == nil {
		return domain.InitialDeal{}
	}
	return domain.InitialDeal{
		Hands:               cloneHands(e.Hands),
		Stock:               slices.Clone(e.Stock),
		TrumpIndicator:      e.TrumpIndicator,
		TrumpSuit:           e.TrumpSuit,
		FirstAttacker:       int(e.FirstAttacker),
		Redeals:             e.Redeals,
		TrumpReselections:   e.TrumpReselections,
		RandomFirstAttacker: e.RandomFirstAttacker,
	}
}

func cloneEvent(e *Event) Event {
	if e == nil {
		return Event{}
	}
	return Event{
		MatchID:  e.MatchID,
		Sequence: e.Sequence,
		Domain:   e.Domain.Clone(),
	}
}

func cloneEvents(events []Event) []Event {
	cloned := make([]Event, len(events))
	for i := range events {
		cloned[i] = cloneEvent(&events[i])
	}
	return cloned
}

func cloneInternalDealEvent(deal *InternalDealEvent) *InternalDealEvent {
	if deal == nil {
		return nil
	}
	cloned := *deal
	cloned.Hands = cloneHands(deal.Hands)
	cloned.Stock = slices.Clone(deal.Stock)
	return &cloned
}

func cloneInternalEvent(e *InternalEvent) InternalEvent {
	if e == nil {
		return InternalEvent{}
	}
	return InternalEvent{
		MatchID:  e.MatchID,
		Sequence: e.Sequence,
		Domain:   e.Domain.Clone(),
		Deal:     cloneInternalDealEvent(e.Deal),
	}
}

func cloneInternalEvents(events []InternalEvent) []InternalEvent {
	cloned := make([]InternalEvent, len(events))
	for i := range events {
		cloned[i] = cloneInternalEvent(&events[i])
	}
	return cloned
}

func cloneHands(hands [][]domain.Card) [][]domain.Card {
	cloned := make([][]domain.Card, len(hands))
	for i, hand := range hands {
		cloned[i] = slices.Clone(hand)
	}
	return cloned
}
