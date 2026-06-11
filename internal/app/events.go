package app

import (
	"context"

	"github.com/vovakirdan/durak/internal/domain"
)

// Event is an application-level sequenced domain event.
type Event struct {
	Sequence uint64
	Domain   domain.Event
}

// EventSink receives sequenced events emitted by active sessions.
type EventSink interface {
	RecordEvent(context.Context, Event) error
}

// InMemoryEventRecorder stores events for tests and future local history wiring.
type InMemoryEventRecorder struct {
	events []Event
}

// NewInMemoryEventRecorder creates an empty event recorder.
func NewInMemoryEventRecorder() *InMemoryEventRecorder {
	return &InMemoryEventRecorder{}
}

// RecordEvent stores event unless context has already been canceled.
func (r *InMemoryEventRecorder) RecordEvent(ctx context.Context, event Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.events = append(r.events, event.clone())
	return nil
}

// Events returns copied recorded events.
func (r *InMemoryEventRecorder) Events() []Event {
	return cloneEvents(r.events)
}

func (e Event) clone() Event {
	return Event{
		Sequence: e.Sequence,
		Domain:   e.Domain.Clone(),
	}
}

func cloneEvents(events []Event) []Event {
	cloned := make([]Event, len(events))
	for i, event := range events {
		cloned[i] = event.clone()
	}
	return cloned
}
