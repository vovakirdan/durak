package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestAnalyzeInternalEventsScoresStoredActions(t *testing.T) {
	events := analysisEvents(t)

	analysis, err := evaluation.AnalyzeInternalEvents(events, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("AnalyzeInternalEvents returned error: %v", err)
	}

	if analysis.MatchID != "analysis-match" {
		t.Fatalf("MatchID = %q, want analysis-match", analysis.MatchID)
	}
	if len(analysis.Moves) != 2 {
		t.Fatalf("moves = %+v, want two analyzed moves", analysis.Moves)
	}
	if analysis.Summary.Moves != 2 {
		t.Fatalf("summary moves = %d, want 2", analysis.Summary.Moves)
	}
	if analysis.Moves[0].Rank != 1 {
		t.Fatalf("first move rank = %d, want best stored attack", analysis.Moves[0].Rank)
	}
	if analysis.Moves[1].Quality == "" {
		t.Fatalf("second move quality is empty")
	}
	if len(analysis.WorstMoves(1)) != 1 {
		t.Fatalf("WorstMoves(1) length mismatch")
	}
}

func analysisEvents(t *testing.T) []app.InternalEvent {
	t.Helper()
	deal := domain.InitialDeal{
		Hands: [][]domain.Card{
			{
				{Rank: domain.Six, Suit: domain.Clubs},
				{Rank: domain.Ace, Suit: domain.Hearts},
				{Rank: domain.King, Suit: domain.Spades},
				{Rank: domain.Queen, Suit: domain.Diamonds},
				{Rank: domain.Jack, Suit: domain.Spades},
				{Rank: domain.Ten, Suit: domain.Diamonds},
			},
			{
				{Rank: domain.Seven, Suit: domain.Clubs},
				{Rank: domain.Eight, Suit: domain.Hearts},
				{Rank: domain.Nine, Suit: domain.Spades},
				{Rank: domain.Ten, Suit: domain.Spades},
				{Rank: domain.Jack, Suit: domain.Diamonds},
				{Rank: domain.Queen, Suit: domain.Spades},
			},
		},
		Stock: []domain.Card{
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Nine, Suit: domain.Hearts},
		},
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}
	match, err := domain.NewMatch(&deal, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	events := appendInternalEvents(nil, match.DrainEvents(), &deal, match.Defender())

	attack := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: deal.Hands[0][0]}
	if err := match.ApplyAction(attack); err != nil {
		t.Fatalf("ApplyAction attack returned error: %v", err)
	}
	events = appendInternalEvents(events, match.DrainEvents(), nil, domain.NoSeat)

	defend := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        deal.Hands[1][0],
		AttackIndex: 0,
	}
	if err := match.ApplyAction(defend); err != nil {
		t.Fatalf("ApplyAction defend returned error: %v", err)
	}
	events = appendInternalEvents(events, match.DrainEvents(), nil, domain.NoSeat)
	return events
}

func appendInternalEvents(
	events []app.InternalEvent,
	domainEvents []domain.Event,
	deal *domain.InitialDeal,
	defender domain.Seat,
) []app.InternalEvent {
	for _, event := range domainEvents {
		internal := app.InternalEvent{
			MatchID:  "analysis-match",
			Sequence: uint64(len(events) + 1),
			Domain:   event,
		}
		if event.Kind == domain.EventKindDeal {
			canonical := app.NewInternalDealEvent(deal, defender)
			internal.Deal = &canonical
		}
		events = append(events, internal)
	}
	return events
}
