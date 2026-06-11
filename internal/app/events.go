package app

import (
	"context"

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

// EventStore appends sequenced events emitted by active sessions.
type EventStore interface {
	AppendEvents(context.Context, []Event) error
}

// InMemoryEventStore stores events for tests and future local history wiring.
type InMemoryEventStore struct {
	events []Event
}

// NewInMemoryEventStore creates an empty event store.
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{}
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
