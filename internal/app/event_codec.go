package app

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

// EventEnvelopeSchemaVersion is the current durable JSON event schema version.
const EventEnvelopeSchemaVersion = 1

// EventVisibility identifies who can safely consume a serialized event.
type EventVisibility string

const (
	// EventVisibilityPublic marks events that do not expose hidden hand contents.
	EventVisibilityPublic EventVisibility = "public"
)

// ErrInvalidEventEnvelope means an event cannot be represented in the stable schema.
var ErrInvalidEventEnvelope = errors.New("invalid event envelope")

// EventEnvelope is the stable JSON shape for persisted or exported match events.
type EventEnvelope struct {
	SchemaVersion int             `json:"schema_version"`
	MatchID       MatchID         `json:"match_id"`
	Sequence      uint64          `json:"sequence"`
	Kind          string          `json:"kind"`
	Visibility    EventVisibility `json:"visibility"`
	Payload       json.RawMessage `json:"payload"`
}

// NewEventEnvelope converts a runtime event into a stable JSON-ready envelope.
func NewEventEnvelope(event *Event) (EventEnvelope, error) {
	if event == nil {
		return EventEnvelope{}, fmt.Errorf("%w: event is nil", ErrInvalidEventEnvelope)
	}
	if event.MatchID == "" {
		return EventEnvelope{}, fmt.Errorf("%w: match id is empty", ErrInvalidEventEnvelope)
	}
	if event.Sequence == 0 {
		return EventEnvelope{}, fmt.Errorf("%w: sequence is zero", ErrInvalidEventEnvelope)
	}

	kind, payload, err := encodeDomainEvent(&event.Domain)
	if err != nil {
		return EventEnvelope{}, err
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return EventEnvelope{}, fmt.Errorf("%w: marshal payload: %w", ErrInvalidEventEnvelope, err)
	}

	return EventEnvelope{
		SchemaVersion: EventEnvelopeSchemaVersion,
		MatchID:       event.MatchID,
		Sequence:      event.Sequence,
		Kind:          kind,
		Visibility:    EventVisibilityPublic,
		Payload:       payloadJSON,
	}, nil
}

// Event converts an envelope back into a runtime event.
func (e *EventEnvelope) Event() (Event, error) {
	if e == nil {
		return Event{}, fmt.Errorf("%w: envelope is nil", ErrInvalidEventEnvelope)
	}
	if e.SchemaVersion != EventEnvelopeSchemaVersion {
		return Event{}, fmt.Errorf("%w: schema version %d", ErrInvalidEventEnvelope, e.SchemaVersion)
	}
	if e.MatchID == "" {
		return Event{}, fmt.Errorf("%w: match id is empty", ErrInvalidEventEnvelope)
	}
	if e.Sequence == 0 {
		return Event{}, fmt.Errorf("%w: sequence is zero", ErrInvalidEventEnvelope)
	}
	if e.Visibility != EventVisibilityPublic {
		return Event{}, fmt.Errorf("%w: visibility %q", ErrInvalidEventEnvelope, e.Visibility)
	}

	event, err := decodeDomainEvent(e.Kind, e.Payload)
	if err != nil {
		return Event{}, err
	}
	return Event{
		MatchID:  e.MatchID,
		Sequence: e.Sequence,
		Domain:   event,
	}, nil
}

// MarshalEventJSON serializes one application event as a stable JSON object.
func MarshalEventJSON(event *Event) ([]byte, error) {
	envelope, err := NewEventEnvelope(event)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal envelope: %w", ErrInvalidEventEnvelope, err)
	}
	return data, nil
}

// UnmarshalEventJSON deserializes one stable JSON event object.
func UnmarshalEventJSON(data []byte) (Event, error) {
	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Event{}, fmt.Errorf("%w: unmarshal envelope: %w", ErrInvalidEventEnvelope, err)
	}
	return (&envelope).Event()
}

func encodeDomainEvent(event *domain.Event) (kind string, payload any, err error) {
	if event == nil {
		return "", nil, fmt.Errorf("%w: domain event is nil", ErrInvalidEventEnvelope)
	}
	switch event.Kind {
	case domain.EventKindMatchStarted:
		if event.Started == nil {
			return "", nil, missingPayload(event.Kind)
		}
		return eventNameMatchStarted, encodeMatchStarted(event.Started), nil
	case domain.EventKindDeal:
		if event.Deal == nil {
			return "", nil, missingPayload(event.Kind)
		}
		payload, err := encodeDeal(event.Deal)
		return eventNameDeal, payload, err
	case domain.EventKindAttack, domain.EventKindDefend, domain.EventKindThrowIn,
		domain.EventKindTransfer, domain.EventKindTake, domain.EventKindFinishDefense,
		domain.EventKindFinishTake:
		if event.Action == nil {
			return "", nil, missingPayload(event.Kind)
		}
		name, ok := eventNameForDomainKind(event.Kind)
		if !ok {
			return "", nil, unknownDomainKind(event.Kind)
		}
		actionName, err := encodeActionKind(event.Action.Action.Kind)
		if err != nil {
			return "", nil, err
		}
		if actionName != name {
			return "", nil, fmt.Errorf("%w: action kind %q does not match event kind %q", ErrInvalidEventEnvelope, actionName, name)
		}
		payload, err := encodeActionEvent(*event.Action)
		return name, payload, err
	case domain.EventKindRefill:
		if event.Refill == nil {
			return "", nil, missingPayload(event.Kind)
		}
		return eventNameRefill, encodeRefill(*event.Refill), nil
	case domain.EventKindRoundEnded:
		if event.RoundEnded == nil {
			return "", nil, missingPayload(event.Kind)
		}
		payload, err := encodeRoundEnded(*event.RoundEnded)
		return eventNameRoundEnded, payload, err
	case domain.EventKindConcede:
		if event.Concede == nil {
			return "", nil, missingPayload(event.Kind)
		}
		return eventNameConcede, encodeConcede(*event.Concede), nil
	case domain.EventKindMatchEnded:
		if event.MatchEnded == nil {
			return "", nil, missingPayload(event.Kind)
		}
		return eventNameMatchEnded, encodeMatchEnded(*event.MatchEnded), nil
	default:
		return "", nil, unknownDomainKind(event.Kind)
	}
}

func decodeDomainEvent(kind string, payload json.RawMessage) (domain.Event, error) {
	switch kind {
	case eventNameMatchStarted:
		event, err := decodeMatchStarted(payload)
		return domain.Event{Kind: domain.EventKindMatchStarted, Started: &event}, err
	case eventNameDeal:
		event, err := decodeDeal(payload)
		return domain.Event{Kind: domain.EventKindDeal, Deal: &event}, err
	case eventNameAttack, eventNameDefend, eventNameThrowIn, eventNameTransfer,
		eventNameTake, eventNameFinishDefense, eventNameFinishTake:
		action, err := decodeActionEvent(payload)
		if err != nil {
			return domain.Event{}, err
		}
		actionName, err := encodeActionKind(action.Action.Kind)
		if err != nil {
			return domain.Event{}, err
		}
		if actionName != kind {
			return domain.Event{}, fmt.Errorf("%w: action kind %q does not match event kind %q", ErrInvalidEventEnvelope, actionName, kind)
		}
		eventKind, ok := domainKindForEventName(kind)
		if !ok {
			return domain.Event{}, unknownEventName(kind)
		}
		return domain.Event{Kind: eventKind, Action: &action}, nil
	case eventNameRefill:
		event, err := decodeRefill(payload)
		return domain.Event{Kind: domain.EventKindRefill, Refill: &event}, err
	case eventNameRoundEnded:
		event, err := decodeRoundEnded(payload)
		return domain.Event{Kind: domain.EventKindRoundEnded, RoundEnded: &event}, err
	case eventNameConcede:
		event, err := decodeConcede(payload)
		return domain.Event{Kind: domain.EventKindConcede, Concede: &event}, err
	case eventNameMatchEnded:
		event, err := decodeMatchEnded(payload)
		return domain.Event{Kind: domain.EventKindMatchEnded, MatchEnded: &event}, err
	default:
		return domain.Event{}, unknownEventName(kind)
	}
}

func missingPayload(kind domain.EventKind) error {
	return fmt.Errorf("%w: missing payload for domain event kind %d", ErrInvalidEventEnvelope, kind)
}

func unknownDomainKind(kind domain.EventKind) error {
	return fmt.Errorf("%w: unknown domain event kind %d", ErrInvalidEventEnvelope, kind)
}

func unknownEventName(kind string) error {
	return fmt.Errorf("%w: unknown event kind %q", ErrInvalidEventEnvelope, kind)
}
