package app

import (
	"encoding/json"
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

func encodeActionEvent(event domain.ActionEvent) (actionEventPayload, error) {
	action, err := encodeAction(event.Action)
	if err != nil {
		return actionEventPayload{}, err
	}
	return actionEventPayload{Action: action}, nil
}

func decodeActionEvent(payload json.RawMessage) (domain.ActionEvent, error) {
	var event actionEventPayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.ActionEvent{}, err
	}
	action, err := decodeAction(event.Action)
	if err != nil {
		return domain.ActionEvent{}, err
	}
	return domain.ActionEvent{Action: action}, nil
}

func encodeAction(action domain.Action) (actionPayload, error) {
	kind, err := encodeActionKind(action.Kind)
	if err != nil {
		return actionPayload{}, err
	}
	payload := actionPayload{
		Kind:        kind,
		Seat:        int(action.Seat),
		AttackIndex: action.AttackIndex,
	}
	if actionHasCard(action.Kind) {
		card, err := encodeCard(action.Card)
		if err != nil {
			return actionPayload{}, err
		}
		payload.Card = &card
	}
	return payload, nil
}

func decodeAction(payload actionPayload) (domain.Action, error) {
	kind, err := decodeActionKind(payload.Kind)
	if err != nil {
		return domain.Action{}, err
	}
	action := domain.Action{
		Kind:        kind,
		Seat:        domain.Seat(payload.Seat),
		AttackIndex: payload.AttackIndex,
	}
	if actionHasCard(kind) {
		if payload.Card == nil {
			return domain.Action{}, fmt.Errorf("%w: missing action card", ErrInvalidEventEnvelope)
		}
		card, err := decodeCard(*payload.Card)
		if err != nil {
			return domain.Action{}, err
		}
		action.Card = card
	}
	return action, nil
}
