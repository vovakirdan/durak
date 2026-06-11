package app_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestReplayInternalEventsReconstructsMatch(t *testing.T) {
	session, events := replayableInternalStream(t)

	result, err := app.ReplayInternalEvents(events, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("ReplayInternalEvents returned error: %v", err)
	}

	if !reflect.DeepEqual(result.Events, sourceDomainEvents(events)) {
		t.Fatalf("replayed events differ from source")
	}
	expectReplayMatchesSession(t, result.Match, session)
}

func TestReplayInternalEventsRejectsSequenceGap(t *testing.T) {
	_, events := replayableInternalStream(t)
	events[2].Sequence = 99

	_, err := app.ReplayInternalEvents(events, domain.DefaultRuleProfile())
	if !errors.Is(err, app.ErrInvalidReplay) {
		t.Fatalf("ReplayInternalEvents error = %v, want ErrInvalidReplay", err)
	}
}

func TestReplayInternalEventsRejectsDerivedMismatch(t *testing.T) {
	_, events := replayableInternalStream(t)
	for i := range events {
		if events[i].Domain.Kind == domain.EventKindRoundEnded {
			events[i].Domain.RoundEnded.SuccessfulDefenses = 99
			break
		}
	}

	_, err := app.ReplayInternalEvents(events, domain.DefaultRuleProfile())
	if !errors.Is(err, app.ErrInvalidReplay) {
		t.Fatalf("ReplayInternalEvents error = %v, want ErrInvalidReplay", err)
	}
}

func TestReplayInternalEventsRequiresCanonicalDeal(t *testing.T) {
	_, events := replayableInternalStream(t)
	events[1].Deal = nil

	_, err := app.ReplayInternalEvents(events, domain.DefaultRuleProfile())
	if !errors.Is(err, app.ErrInvalidReplay) {
		t.Fatalf("ReplayInternalEvents error = %v, want ErrInvalidReplay", err)
	}
}

func replayableInternalStream(t *testing.T) (*app.Session, []app.InternalEvent) {
	t.Helper()
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	deal := domain.InitialDeal{
		Hands: [][]domain.Card{
			{attack},
			{defense},
		},
		Stock: []domain.Card{
			{Rank: domain.Eight, Suit: domain.Clubs},
			{Rank: domain.Nine, Suit: domain.Clubs},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Clubs},
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Diamonds},
		},
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}
	store := app.NewInMemoryInternalEventStore()
	session, err := app.NewSessionWithOptions(context.Background(), mustMatchFromDeal(t, deal), app.SessionOptions{
		MatchID:            testMatchID,
		InternalEventStore: store,
		InitialDeal:        &deal,
	})
	if err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	return session, store.Events()
}

func sourceDomainEvents(events []app.InternalEvent) []domain.Event {
	source := make([]domain.Event, len(events))
	for i := range events {
		source[i] = events[i].Domain.Clone()
	}
	return source
}

func expectReplayMatchesSession(t *testing.T, match *domain.Match, session *app.Session) {
	t.Helper()
	view := session.ViewForSeat(domain.Seat(0))
	if match.Phase() != view.Phase {
		t.Fatalf("phase = %v, want %v", match.Phase(), view.Phase)
	}
	if match.Attacker() != view.Attacker || match.Defender() != view.Defender {
		t.Fatalf("roles = %d/%d, want %d/%d", match.Attacker(), match.Defender(), view.Attacker, view.Defender)
	}
	if match.StockCount() != view.StockCount || match.DiscardCount() != view.DiscardCount {
		t.Fatalf("counts = stock %d discard %d, want stock %d discard %d",
			match.StockCount(), match.DiscardCount(), view.StockCount, view.DiscardCount)
	}
	if !slices.Equal(match.Table(), view.Table) {
		t.Fatalf("table = %v, want %v", match.Table(), view.Table)
	}
	for seat := range view.HandSizes {
		domainSeat := domain.Seat(seat)
		if match.HandSize(domainSeat) != view.HandSizes[seat] {
			t.Fatalf("seat %d hand size = %d, want %d", seat, match.HandSize(domainSeat), view.HandSizes[seat])
		}
		wantHand := session.DecisionContext(domainSeat).Hand
		if !slices.Equal(match.Hand(domainSeat), wantHand) {
			t.Fatalf("seat %d hand = %v, want %v", seat, match.Hand(domainSeat), wantHand)
		}
	}
}
