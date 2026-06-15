package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestMarshalEventJSONRoundTripsAllEventKinds(t *testing.T) {
	events := []app.Event{
		testEvent(domain.Event{
			Kind:    domain.EventKindMatchStarted,
			Started: &domain.MatchStartedEvent{PlayerCount: 2, RuleProfile: "classic"},
		}),
		testEvent(domain.Event{
			Kind: domain.EventKindDeal,
			Deal: &domain.DealEvent{
				TrumpIndicator:      domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
				TrumpSuit:           domain.Hearts,
				FirstAttacker:       domain.Seat(0),
				Defender:            domain.Seat(1),
				HandSizes:           []int{6, 6},
				StockCount:          24,
				Redeals:             1,
				TrumpReselections:   2,
				RandomFirstAttacker: true,
			},
		}),
		testActionEvent(domain.EventKindAttack, domain.ActionKindAttack, domain.Seat(0), domain.Card{Rank: domain.Six, Suit: domain.Clubs}, 0),
		testActionEvent(domain.EventKindDefend, domain.ActionKindDefend, domain.Seat(1), domain.Card{Rank: domain.Seven, Suit: domain.Clubs}, 1),
		testActionEvent(domain.EventKindThrowIn, domain.ActionKindThrowIn, domain.Seat(0), domain.Card{Rank: domain.Six, Suit: domain.Diamonds}, 0),
		testActionEvent(domain.EventKindPassThrowIn, domain.ActionKindPassThrowIn, domain.Seat(2), domain.Card{}, 0),
		testActionEvent(domain.EventKindTransfer, domain.ActionKindTransfer, domain.Seat(1), domain.Card{Rank: domain.Six, Suit: domain.Spades}, 0),
		testActionEvent(domain.EventKindTake, domain.ActionKindTake, domain.Seat(1), domain.Card{}, 0),
		testActionEvent(domain.EventKindFinishDefense, domain.ActionKindFinishDefense, domain.Seat(0), domain.Card{}, 0),
		testActionEvent(domain.EventKindFinishTake, domain.ActionKindFinishTake, domain.Seat(0), domain.Card{}, 0),
		testEvent(domain.Event{
			Kind:   domain.EventKindRefill,
			Refill: &domain.RefillEvent{Seat: domain.Seat(0), Drawn: 3, HandSize: 6, StockCount: 12},
		}),
		testEvent(domain.Event{
			Kind: domain.EventKindRoundEnded,
			RoundEnded: &domain.RoundEndedEvent{
				Outcome:            domain.RoundOutcomeDefense,
				Attacker:           domain.Seat(0),
				Defender:           domain.Seat(1),
				Cards:              []domain.Card{{Rank: domain.Six, Suit: domain.Clubs}, {Rank: domain.Seven, Suit: domain.Clubs}},
				NextAttacker:       domain.Seat(1),
				NextDefender:       domain.Seat(0),
				SuccessfulDefenses: 1,
			},
		}),
		testEvent(domain.Event{
			Kind:    domain.EventKindConcede,
			Concede: &domain.ConcedeEvent{Seat: domain.Seat(1), Winner: domain.Seat(0)},
		}),
		testEvent(domain.Event{
			Kind:       domain.EventKindMatchEnded,
			MatchEnded: &domain.MatchEndedEvent{Winner: domain.NoSeat, Loser: domain.NoSeat, Draw: true},
		}),
	}

	for _, event := range events {
		data, err := app.MarshalEventJSON(&event)
		if err != nil {
			t.Fatalf("MarshalEventJSON(%v) returned error: %v", event.Domain.Kind, err)
		}
		decoded, err := app.UnmarshalEventJSON(data)
		if err != nil {
			t.Fatalf("UnmarshalEventJSON(%v) returned error: %v", event.Domain.Kind, err)
		}
		if !reflect.DeepEqual(decoded, event) {
			t.Fatalf("round trip = %+v, want %+v", decoded, event)
		}
	}
}

func TestMarshalEventJSONUsesStableEnvelope(t *testing.T) {
	event := testActionEvent(
		domain.EventKindAttack,
		domain.ActionKindAttack,
		domain.Seat(0),
		domain.Card{Rank: domain.Six, Suit: domain.Clubs},
		0,
	)

	data, err := app.MarshalEventJSON(&event)
	if err != nil {
		t.Fatalf("MarshalEventJSON returned error: %v", err)
	}

	var envelope struct {
		SchemaVersion int             `json:"schema_version"`
		MatchID       app.MatchID     `json:"match_id"`
		Sequence      uint64          `json:"sequence"`
		Kind          string          `json:"kind"`
		Visibility    string          `json:"visibility"`
		Payload       json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if envelope.SchemaVersion != app.EventEnvelopeSchemaVersion {
		t.Fatalf("schema version = %d, want %d", envelope.SchemaVersion, app.EventEnvelopeSchemaVersion)
	}
	if envelope.MatchID != testMatchID || envelope.Sequence != 42 {
		t.Fatalf("match/sequence = %q/%d, want %q/42", envelope.MatchID, envelope.Sequence, testMatchID)
	}
	if envelope.Kind != "attack" || envelope.Visibility != "public" {
		t.Fatalf("kind/visibility = %q/%q, want attack/public", envelope.Kind, envelope.Visibility)
	}
	if bytes.Contains(data, []byte(`"Kind"`)) || bytes.Contains(data, []byte(`"MatchID"`)) {
		t.Fatalf("json uses Go field names: %s", data)
	}
	if !bytes.Contains(envelope.Payload, []byte(`"rank":"6"`)) || !bytes.Contains(envelope.Payload, []byte(`"suit":"C"`)) {
		t.Fatalf("payload = %s, want stable card rank/suit", envelope.Payload)
	}
}

func TestMarshalEventJSONIncludesMatchConfigIdentity(t *testing.T) {
	identity := app.MatchConfigIdentity{
		SchemaVersion: app.CurrentMatchConfigSchemaVersion,
		RulePreset:    app.RulePresetDefault,
		RuleProfile:   "default",
		Hash:          "sha256:1234567890abcdef",
	}
	event := testEvent(domain.Event{
		Kind:    domain.EventKindMatchStarted,
		Started: &domain.MatchStartedEvent{PlayerCount: 2, RuleProfile: "default"},
	})
	event.ConfigIdentity = identity

	data, err := app.MarshalEventJSON(&event)
	if err != nil {
		t.Fatalf("MarshalEventJSON returned error: %v", err)
	}

	var envelope struct {
		Payload json.RawMessage `json:"payload"`
	}
	if unmarshalErr := json.Unmarshal(data, &envelope); unmarshalErr != nil {
		t.Fatalf("json.Unmarshal envelope returned error: %v", unmarshalErr)
	}
	var payload struct {
		Config struct {
			SchemaVersion int    `json:"schema_version"`
			RulePreset    string `json:"rule_preset"`
			RuleProfile   string `json:"rule_profile"`
			Hash          string `json:"hash"`
		} `json:"config"`
	}
	if unmarshalErr := json.Unmarshal(envelope.Payload, &payload); unmarshalErr != nil {
		t.Fatalf("json.Unmarshal payload returned error: %v", unmarshalErr)
	}
	if payload.Config.SchemaVersion != identity.SchemaVersion ||
		payload.Config.RulePreset != identity.RulePreset ||
		payload.Config.RuleProfile != identity.RuleProfile ||
		payload.Config.Hash != identity.Hash {
		t.Fatalf("payload config = %+v, want %+v", payload.Config, identity)
	}

	decoded, err := app.UnmarshalEventJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalEventJSON returned error: %v", err)
	}
	if decoded.ConfigIdentity != identity {
		t.Fatalf("decoded ConfigIdentity = %+v, want %+v", decoded.ConfigIdentity, identity)
	}
}

func TestMarshalInternalEventJSONRoundTripsFullDeal(t *testing.T) {
	deal := app.InternalDealEvent{
		Hands: [][]domain.Card{
			{{Rank: domain.Six, Suit: domain.Clubs}},
			{{Rank: domain.Seven, Suit: domain.Hearts}},
		},
		Stock: []domain.Card{
			{Rank: domain.Eight, Suit: domain.Spades},
			{Rank: domain.Nine, Suit: domain.Diamonds},
		},
		TrumpIndicator:      domain.Card{Rank: domain.Nine, Suit: domain.Diamonds},
		TrumpSuit:           domain.Diamonds,
		FirstAttacker:       domain.Seat(0),
		Defender:            domain.Seat(1),
		Redeals:             1,
		TrumpReselections:   2,
		RandomFirstAttacker: true,
	}
	publicDeal := deal.PublicDeal()
	event := app.InternalEvent{
		MatchID:  testMatchID,
		Sequence: 42,
		Domain: domain.Event{
			Kind: domain.EventKindDeal,
			Deal: &publicDeal,
		},
		Deal: &deal,
	}

	data, err := app.MarshalInternalEventJSON(&event)
	if err != nil {
		t.Fatalf("MarshalInternalEventJSON returned error: %v", err)
	}
	if !bytes.Contains(data, []byte(`"visibility":"internal"`)) {
		t.Fatalf("internal event visibility not encoded: %s", data)
	}
	if !bytes.Contains(data, []byte(`"hands"`)) || !bytes.Contains(data, []byte(`"stock"`)) {
		t.Fatalf("internal deal omitted hidden setup state: %s", data)
	}
	if _, publicErr := app.UnmarshalEventJSON(data); !errors.Is(publicErr, app.ErrInvalidEventEnvelope) {
		t.Fatalf("public unmarshal error = %v, want ErrInvalidEventEnvelope", publicErr)
	}

	decoded, err := app.UnmarshalInternalEventJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalInternalEventJSON returned error: %v", err)
	}
	if !reflect.DeepEqual(decoded, event) {
		t.Fatalf("internal round trip = %+v, want %+v", decoded, event)
	}
}

func TestMarshalInternalEventJSONRoundTripsAction(t *testing.T) {
	event := app.InternalEvent{
		MatchID:  testMatchID,
		Sequence: 43,
		Domain: domain.Event{
			Kind: domain.EventKindAttack,
			Action: &domain.ActionEvent{
				Action: domain.Action{
					Kind: domain.ActionKindAttack,
					Seat: domain.Seat(0),
					Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs},
				},
			},
		},
	}

	data, err := app.MarshalInternalEventJSON(&event)
	if err != nil {
		t.Fatalf("MarshalInternalEventJSON returned error: %v", err)
	}
	decoded, err := app.UnmarshalInternalEventJSON(data)
	if err != nil {
		t.Fatalf("UnmarshalInternalEventJSON returned error: %v", err)
	}
	if !reflect.DeepEqual(decoded, event) {
		t.Fatalf("internal round trip = %+v, want %+v", decoded, event)
	}
}

func TestUnmarshalEventJSONRejectsInvalidEnvelope(t *testing.T) {
	tests := []string{
		`{"schema_version":2,"match_id":"match","sequence":1,"kind":"match_started","visibility":"public","payload":{}}`,
		`{"schema_version":1,"sequence":1,"kind":"match_started","visibility":"public","payload":{}}`,
		`{"schema_version":1,"match_id":"match","sequence":0,"kind":"match_started","visibility":"public","payload":{}}`,
		`{"schema_version":1,"match_id":"match","sequence":1,"kind":"unknown","visibility":"public","payload":{}}`,
		`{"schema_version":1,"match_id":"match","sequence":1,"kind":"match_started","visibility":"private","payload":{}}`,
		`{"schema_version":1,"match_id":"match","sequence":1,"kind":"attack","visibility":"public","payload":{"action":{"kind":"defend","seat":0,"card":{"rank":"6","suit":"C"}}}}`,
	}

	for _, input := range tests {
		_, err := app.UnmarshalEventJSON([]byte(input))
		if !errors.Is(err, app.ErrInvalidEventEnvelope) {
			t.Fatalf("UnmarshalEventJSON(%s) error = %v, want ErrInvalidEventEnvelope", input, err)
		}
	}
}

func TestUnmarshalInternalEventJSONRejectsInvalidEnvelope(t *testing.T) {
	tests := []string{
		`{"schema_version":1,"match_id":"match","sequence":1,"kind":"match_started","visibility":"public","payload":{}}`,
		`{"schema_version":1,"match_id":"match","sequence":1,"kind":"deal","visibility":"internal","payload":{"hand_sizes":[1,1]}}`,
	}

	for _, input := range tests {
		_, err := app.UnmarshalInternalEventJSON([]byte(input))
		if !errors.Is(err, app.ErrInvalidEventEnvelope) {
			t.Fatalf("UnmarshalInternalEventJSON(%s) error = %v, want ErrInvalidEventEnvelope", input, err)
		}
	}
}

func TestMarshalEventJSONRejectsInvalidRuntimeEvent(t *testing.T) {
	events := []app.Event{
		testEvent(domain.Event{Kind: domain.EventKindAttack}),
		testEvent(domain.Event{
			Kind: domain.EventKindAttack,
			Action: &domain.ActionEvent{
				Action: domain.Action{Kind: domain.ActionKindUnknown, Seat: domain.Seat(0)},
			},
		}),
		testEvent(domain.Event{
			Kind: domain.EventKindAttack,
			Action: &domain.ActionEvent{
				Action: domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0)},
			},
		}),
		testEvent(domain.Event{
			Kind: domain.EventKindRoundEnded,
			RoundEnded: &domain.RoundEndedEvent{
				Outcome: domain.RoundOutcomeUnknown,
			},
		}),
		testEvent(domain.Event{
			Kind: domain.EventKindAttack,
			Action: &domain.ActionEvent{
				Action: domain.Action{
					Kind: domain.ActionKindDefend,
					Seat: domain.Seat(1),
					Card: domain.Card{Rank: domain.Seven, Suit: domain.Clubs},
				},
			},
		}),
	}

	for _, event := range events {
		_, err := app.MarshalEventJSON(&event)
		if !errors.Is(err, app.ErrInvalidEventEnvelope) {
			t.Fatalf("MarshalEventJSON(%+v) error = %v, want ErrInvalidEventEnvelope", event.Domain, err)
		}
	}
}

func TestMarshalInternalEventJSONRejectsInvalidRuntimeEvent(t *testing.T) {
	events := []app.InternalEvent{
		{MatchID: testMatchID, Sequence: 42, Domain: domain.Event{Kind: domain.EventKindDeal}},
		{MatchID: testMatchID, Sequence: 42, Domain: domain.Event{Kind: domain.EventKindAttack}},
	}

	for _, event := range events {
		_, err := app.MarshalInternalEventJSON(&event)
		if !errors.Is(err, app.ErrInvalidEventEnvelope) {
			t.Fatalf("MarshalInternalEventJSON(%+v) error = %v, want ErrInvalidEventEnvelope", event, err)
		}
	}
}

func testEvent(event domain.Event) app.Event {
	return app.Event{
		MatchID:  testMatchID,
		Sequence: 42,
		Domain:   event,
	}
}

func testActionEvent(kind domain.EventKind, actionKind domain.ActionKind, seat domain.Seat, card domain.Card, attackIndex int) app.Event {
	return testEvent(domain.Event{
		Kind: kind,
		Action: &domain.ActionEvent{
			Action: domain.Action{
				Kind:        actionKind,
				Seat:        seat,
				Card:        card,
				AttackIndex: attackIndex,
			},
		},
	})
}
