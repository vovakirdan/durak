package app

import (
	"encoding/json"
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

const (
	eventNameMatchStarted  = "match_started"
	eventNameDeal          = "deal"
	eventNameAttack        = "attack"
	eventNameDefend        = "defend"
	eventNameThrowIn       = "throw_in"
	eventNameTransfer      = "transfer"
	eventNameTake          = "take"
	eventNameFinishDefense = "finish_defense"
	eventNameFinishTake    = "finish_take"
	eventNameRefill        = "refill"
	eventNameRoundEnded    = "round_ended"
	eventNameConcede       = "concede"
	eventNameMatchEnded    = "match_ended"
)

const (
	roundOutcomeDefense = "defense"
	roundOutcomeTake    = "take"
)

type matchStartedPayload struct {
	PlayerCount int    `json:"player_count"`
	RuleProfile string `json:"rule_profile"`
}

type dealPayload struct {
	TrumpIndicator      cardPayload `json:"trump_indicator"`
	TrumpSuit           string      `json:"trump_suit"`
	FirstAttacker       int         `json:"first_attacker"`
	Defender            int         `json:"defender"`
	HandSizes           []int       `json:"hand_sizes"`
	StockCount          int         `json:"stock_count"`
	Redeals             int         `json:"redeals"`
	TrumpReselections   int         `json:"trump_reselections"`
	RandomFirstAttacker bool        `json:"random_first_attacker"`
}

type actionEventPayload struct {
	Action actionPayload `json:"action"`
}

type actionPayload struct {
	Kind        string       `json:"kind"`
	Seat        int          `json:"seat"`
	Card        *cardPayload `json:"card,omitempty"`
	AttackIndex int          `json:"attack_index,omitempty"`
}

type refillPayload struct {
	Seat       int `json:"seat"`
	Drawn      int `json:"drawn"`
	HandSize   int `json:"hand_size"`
	StockCount int `json:"stock_count"`
}

type roundEndedPayload struct {
	Outcome            string        `json:"outcome"`
	Attacker           int           `json:"attacker"`
	Defender           int           `json:"defender"`
	Cards              []cardPayload `json:"cards"`
	NextAttacker       int           `json:"next_attacker"`
	NextDefender       int           `json:"next_defender"`
	SuccessfulDefenses int           `json:"successful_defenses"`
}

type concedePayload struct {
	Seat   int `json:"seat"`
	Winner int `json:"winner"`
}

type matchEndedPayload struct {
	Winner int  `json:"winner"`
	Loser  int  `json:"loser"`
	Draw   bool `json:"draw"`
}

type cardPayload struct {
	Rank string `json:"rank"`
	Suit string `json:"suit"`
}

func encodeMatchStarted(event *domain.MatchStartedEvent) matchStartedPayload {
	return matchStartedPayload{
		PlayerCount: event.PlayerCount,
		RuleProfile: event.RuleProfile,
	}
}

func decodeMatchStarted(payload json.RawMessage) (domain.MatchStartedEvent, error) {
	var event matchStartedPayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.MatchStartedEvent{}, err
	}
	return domain.MatchStartedEvent{
		PlayerCount: event.PlayerCount,
		RuleProfile: event.RuleProfile,
	}, nil
}

func encodeDeal(event *domain.DealEvent) (dealPayload, error) {
	trumpIndicator, err := encodeCard(event.TrumpIndicator)
	if err != nil {
		return dealPayload{}, err
	}
	trumpSuit, err := encodeSuit(event.TrumpSuit)
	if err != nil {
		return dealPayload{}, err
	}
	return dealPayload{
		TrumpIndicator:      trumpIndicator,
		TrumpSuit:           trumpSuit,
		FirstAttacker:       int(event.FirstAttacker),
		Defender:            int(event.Defender),
		HandSizes:           append([]int(nil), event.HandSizes...),
		StockCount:          event.StockCount,
		Redeals:             event.Redeals,
		TrumpReselections:   event.TrumpReselections,
		RandomFirstAttacker: event.RandomFirstAttacker,
	}, nil
}

func decodeDeal(payload json.RawMessage) (domain.DealEvent, error) {
	var event dealPayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.DealEvent{}, err
	}
	trumpIndicator, err := decodeCard(event.TrumpIndicator)
	if err != nil {
		return domain.DealEvent{}, err
	}
	trumpSuit, err := decodeSuit(event.TrumpSuit)
	if err != nil {
		return domain.DealEvent{}, err
	}
	return domain.DealEvent{
		TrumpIndicator:      trumpIndicator,
		TrumpSuit:           trumpSuit,
		FirstAttacker:       domain.Seat(event.FirstAttacker),
		Defender:            domain.Seat(event.Defender),
		HandSizes:           append([]int(nil), event.HandSizes...),
		StockCount:          event.StockCount,
		Redeals:             event.Redeals,
		TrumpReselections:   event.TrumpReselections,
		RandomFirstAttacker: event.RandomFirstAttacker,
	}, nil
}

func encodeRefill(event domain.RefillEvent) refillPayload {
	return refillPayload{
		Seat:       int(event.Seat),
		Drawn:      event.Drawn,
		HandSize:   event.HandSize,
		StockCount: event.StockCount,
	}
}

func decodeRefill(payload json.RawMessage) (domain.RefillEvent, error) {
	var event refillPayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.RefillEvent{}, err
	}
	return domain.RefillEvent{
		Seat:       domain.Seat(event.Seat),
		Drawn:      event.Drawn,
		HandSize:   event.HandSize,
		StockCount: event.StockCount,
	}, nil
}

func encodeConcede(event domain.ConcedeEvent) concedePayload {
	return concedePayload{
		Seat:   int(event.Seat),
		Winner: int(event.Winner),
	}
}

func decodeConcede(payload json.RawMessage) (domain.ConcedeEvent, error) {
	var event concedePayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.ConcedeEvent{}, err
	}
	return domain.ConcedeEvent{
		Seat:   domain.Seat(event.Seat),
		Winner: domain.Seat(event.Winner),
	}, nil
}

func encodeMatchEnded(event domain.MatchEndedEvent) matchEndedPayload {
	return matchEndedPayload{
		Winner: int(event.Winner),
		Loser:  int(event.Loser),
		Draw:   event.Draw,
	}
}

func decodeMatchEnded(payload json.RawMessage) (domain.MatchEndedEvent, error) {
	var event matchEndedPayload
	if err := decodePayload(payload, &event); err != nil {
		return domain.MatchEndedEvent{}, err
	}
	return domain.MatchEndedEvent{
		Winner: domain.Seat(event.Winner),
		Loser:  domain.Seat(event.Loser),
		Draw:   event.Draw,
	}, nil
}

func decodePayload(payload json.RawMessage, target any) error {
	if len(payload) == 0 {
		return fmt.Errorf("%w: payload is empty", ErrInvalidEventEnvelope)
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("%w: decode payload: %w", ErrInvalidEventEnvelope, err)
	}
	return nil
}
