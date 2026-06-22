package client

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestStateFromDecisionProjectsTransportDTO(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:           domain.Seat(1),
			Phase:          domain.MatchPhaseDefense,
			Attacker:       domain.Seat(0),
			Defender:       domain.Seat(1),
			TrumpSuit:      domain.Hearts,
			TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
			Table: []domain.TablePair{
				{Attack: domain.Card{Rank: domain.Six, Suit: domain.Clubs}},
				{
					Attack:   domain.Card{Rank: domain.Seven, Suit: domain.Diamonds},
					Defense:  domain.Card{Rank: domain.Eight, Suit: domain.Diamonds},
					Defended: true,
				},
			},
			HandSizes:    []int{5, 6},
			StockCount:   20,
			DiscardCount: 2,
			Winner:       domain.NoSeat,
			Loser:        domain.NoSeat,
		},
		Hand: []domain.Card{{Rank: domain.Seven, Suit: domain.Clubs}},
		LegalActions: []domain.Action{
			{
				Kind:        domain.ActionKindDefend,
				Seat:        domain.Seat(1),
				Card:        domain.Card{Rank: domain.Seven, Suit: domain.Clubs},
				AttackIndex: 0,
			},
			{Kind: domain.ActionKindTake, Seat: domain.Seat(1)},
		},
	}

	state := StateFromDecision("match-1", 7, &decision)

	if state.MatchID != "match-1" || state.Version != 7 || state.Phase != "defense" {
		t.Fatalf("state identity = %+v, want match-1 version 7 defense", state)
	}
	if state.TrumpIndicator.Code != "9H" || state.TrumpSuit != "H" {
		t.Fatalf("trump = %s/%s, want 9H/H", state.TrumpIndicator.Code, state.TrumpSuit)
	}
	if len(state.Table) != 2 || state.Table[0].Defense != nil || state.Table[1].Defense.Code != "8D" {
		t.Fatalf("table = %+v, want one open and one defended pair", state.Table)
	}
	if len(state.LegalActions) != 2 {
		t.Fatalf("legal actions = %+v, want two actions", state.LegalActions)
	}
	if got := state.LegalActions[0]; got.ID != "1" || got.Kind != "defend" || got.Label != "defend 1 7C" {
		t.Fatalf("first action = %+v, want defend DTO", got)
	}
	if got := state.LegalActions[1]; got.ID != "2" || got.Kind != "take" || got.Card != nil {
		t.Fatalf("second action = %+v, want take DTO without card", got)
	}
}

func TestStateFromDecisionReportsCompleteResult(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Phase:  domain.MatchPhaseComplete,
			Winner: domain.Seat(1),
			Loser:  domain.Seat(0),
		},
	}
	state := StateFromDecision("match-1", 1, &decision)

	if state.Result != "winner:1 loser:0" {
		t.Fatalf("Result = %q, want winner/loser summary", state.Result)
	}
}
