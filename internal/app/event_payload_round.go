package app

import (
	"encoding/json"

	"github.com/vovakirdan/durak/internal/domain"
)

func encodeRoundEnded(event domain.RoundEndedEvent) (roundEndedPayload, error) {
	outcome, err := encodeRoundOutcome(event.Outcome)
	if err != nil {
		return roundEndedPayload{}, err
	}
	cards, err := encodeCards(event.Cards)
	if err != nil {
		return roundEndedPayload{}, err
	}
	return roundEndedPayload{
		Outcome:            outcome,
		Attacker:           int(event.Attacker),
		Defender:           int(event.Defender),
		Cards:              cards,
		NextAttacker:       int(event.NextAttacker),
		NextDefender:       int(event.NextDefender),
		SuccessfulDefenses: event.SuccessfulDefenses,
	}, nil
}

func decodeRoundEnded(payload json.RawMessage) (domain.RoundEndedEvent, error) {
	var event roundEndedPayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.RoundEndedEvent{}, err
	}
	outcome, err := decodeRoundOutcome(event.Outcome)
	if err != nil {
		return domain.RoundEndedEvent{}, err
	}
	cards, err := decodeCards(event.Cards)
	if err != nil {
		return domain.RoundEndedEvent{}, err
	}
	return domain.RoundEndedEvent{
		Outcome:            outcome,
		Attacker:           domain.Seat(event.Attacker),
		Defender:           domain.Seat(event.Defender),
		Cards:              cards,
		NextAttacker:       domain.Seat(event.NextAttacker),
		NextDefender:       domain.Seat(event.NextDefender),
		SuccessfulDefenses: event.SuccessfulDefenses,
	}, nil
}
