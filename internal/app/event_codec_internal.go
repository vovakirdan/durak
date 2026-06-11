package app

import (
	"encoding/json"
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

type internalDealPayload struct {
	Hands               [][]cardPayload `json:"hands"`
	Stock               []cardPayload   `json:"stock"`
	TrumpIndicator      cardPayload     `json:"trump_indicator"`
	TrumpSuit           string          `json:"trump_suit"`
	FirstAttacker       int             `json:"first_attacker"`
	Defender            int             `json:"defender"`
	Redeals             int             `json:"redeals"`
	TrumpReselections   int             `json:"trump_reselections"`
	RandomFirstAttacker bool            `json:"random_first_attacker"`
}

// NewInternalEventEnvelope converts a canonical event into a stable envelope.
func NewInternalEventEnvelope(event *InternalEvent) (EventEnvelope, error) {
	if event == nil {
		return EventEnvelope{}, fmt.Errorf("%w: internal event is nil", ErrInvalidEventEnvelope)
	}
	if event.MatchID == "" {
		return EventEnvelope{}, fmt.Errorf("%w: match id is empty", ErrInvalidEventEnvelope)
	}
	if event.Sequence == 0 {
		return EventEnvelope{}, fmt.Errorf("%w: sequence is zero", ErrInvalidEventEnvelope)
	}

	kind, payload, err := encodeInternalEvent(event)
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
		Visibility:    EventVisibilityInternal,
		Payload:       payloadJSON,
	}, nil
}

// InternalEvent converts an internal envelope back into a runtime event.
func (e *EventEnvelope) InternalEvent() (InternalEvent, error) {
	if e == nil {
		return InternalEvent{}, fmt.Errorf("%w: envelope is nil", ErrInvalidEventEnvelope)
	}
	if e.SchemaVersion != EventEnvelopeSchemaVersion {
		return InternalEvent{}, fmt.Errorf("%w: schema version %d", ErrInvalidEventEnvelope, e.SchemaVersion)
	}
	if e.MatchID == "" {
		return InternalEvent{}, fmt.Errorf("%w: match id is empty", ErrInvalidEventEnvelope)
	}
	if e.Sequence == 0 {
		return InternalEvent{}, fmt.Errorf("%w: sequence is zero", ErrInvalidEventEnvelope)
	}
	if e.Visibility != EventVisibilityInternal {
		return InternalEvent{}, fmt.Errorf("%w: visibility %q", ErrInvalidEventEnvelope, e.Visibility)
	}

	event, err := decodeInternalEvent(e.Kind, e.Payload)
	if err != nil {
		return InternalEvent{}, err
	}
	event.MatchID = e.MatchID
	event.Sequence = e.Sequence
	return event, nil
}

// MarshalInternalEventJSON serializes one canonical event as a stable JSON object.
func MarshalInternalEventJSON(event *InternalEvent) ([]byte, error) {
	envelope, err := NewInternalEventEnvelope(event)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal envelope: %w", ErrInvalidEventEnvelope, err)
	}
	return data, nil
}

// UnmarshalInternalEventJSON deserializes one stable canonical event object.
func UnmarshalInternalEventJSON(data []byte) (InternalEvent, error) {
	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return InternalEvent{}, fmt.Errorf("%w: unmarshal envelope: %w", ErrInvalidEventEnvelope, err)
	}
	return (&envelope).InternalEvent()
}

func encodeInternalEvent(event *InternalEvent) (kind string, payload any, err error) {
	if event.Domain.Kind == domain.EventKindDeal {
		if event.Deal == nil {
			return "", nil, fmt.Errorf("%w: missing internal deal", ErrInvalidEventEnvelope)
		}
		payload, err := encodeInternalDeal(event.Deal)
		return eventNameDeal, payload, err
	}
	return encodeDomainEvent(&event.Domain)
}

func decodeInternalEvent(kind string, payload json.RawMessage) (InternalEvent, error) {
	if kind == eventNameDeal {
		deal, err := decodeInternalDeal(payload)
		if err != nil {
			return InternalEvent{}, err
		}
		publicDeal := deal.PublicDeal()
		return InternalEvent{
			Domain: domain.Event{
				Kind: domain.EventKindDeal,
				Deal: &publicDeal,
			},
			Deal: &deal,
		}, nil
	}
	event, err := decodeDomainEvent(kind, payload)
	if err != nil {
		return InternalEvent{}, err
	}
	return InternalEvent{Domain: event}, nil
}

func encodeInternalDeal(event *InternalDealEvent) (internalDealPayload, error) {
	trumpIndicator, err := encodeCard(event.TrumpIndicator)
	if err != nil {
		return internalDealPayload{}, err
	}
	trumpSuit, err := encodeSuit(event.TrumpSuit)
	if err != nil {
		return internalDealPayload{}, err
	}
	hands, err := encodeHands(event.Hands)
	if err != nil {
		return internalDealPayload{}, err
	}
	stock, err := encodeCards(event.Stock)
	if err != nil {
		return internalDealPayload{}, err
	}
	return internalDealPayload{
		Hands:               hands,
		Stock:               stock,
		TrumpIndicator:      trumpIndicator,
		TrumpSuit:           trumpSuit,
		FirstAttacker:       int(event.FirstAttacker),
		Defender:            int(event.Defender),
		Redeals:             event.Redeals,
		TrumpReselections:   event.TrumpReselections,
		RandomFirstAttacker: event.RandomFirstAttacker,
	}, nil
}

func decodeInternalDeal(payload json.RawMessage) (InternalDealEvent, error) {
	var event internalDealPayload
	if err := decodePayload(payload, &event); err != nil {
		return InternalDealEvent{}, err
	}
	hands, err := decodeHands(event.Hands)
	if err != nil {
		return InternalDealEvent{}, err
	}
	stock, err := decodeCards(event.Stock)
	if err != nil {
		return InternalDealEvent{}, err
	}
	trumpIndicator, err := decodeCard(event.TrumpIndicator)
	if err != nil {
		return InternalDealEvent{}, err
	}
	trumpSuit, err := decodeSuit(event.TrumpSuit)
	if err != nil {
		return InternalDealEvent{}, err
	}
	return InternalDealEvent{
		Hands:               hands,
		Stock:               stock,
		TrumpIndicator:      trumpIndicator,
		TrumpSuit:           trumpSuit,
		FirstAttacker:       domain.Seat(event.FirstAttacker),
		Defender:            domain.Seat(event.Defender),
		Redeals:             event.Redeals,
		TrumpReselections:   event.TrumpReselections,
		RandomFirstAttacker: event.RandomFirstAttacker,
	}, nil
}

func encodeHands(hands [][]domain.Card) ([][]cardPayload, error) {
	encoded := make([][]cardPayload, len(hands))
	for i, hand := range hands {
		cards, err := encodeCards(hand)
		if err != nil {
			return nil, err
		}
		encoded[i] = cards
	}
	return encoded, nil
}

func decodeHands(hands [][]cardPayload) ([][]domain.Card, error) {
	decoded := make([][]domain.Card, len(hands))
	for i, hand := range hands {
		cards, err := decodeCards(hand)
		if err != nil {
			return nil, err
		}
		decoded[i] = cards
	}
	return decoded, nil
}
