package app_test

import (
	"context"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestInMemoryEventStoreReturnsCopiesByMatch(t *testing.T) {
	store := app.NewInMemoryEventStore()
	first := app.MatchID("match-1")
	second := app.MatchID("match-2")
	events := []app.Event{
		{
			MatchID:  first,
			Sequence: 1,
			Domain: domain.Event{
				Kind: domain.EventKindDeal,
				Deal: &domain.DealEvent{
					HandSizes: []int{1, 1},
				},
			},
		},
		{
			MatchID:  second,
			Sequence: 1,
			Domain:   domain.Event{Kind: domain.EventKindMatchStarted},
		},
	}

	if err := store.AppendEvents(context.Background(), events); err != nil {
		t.Fatalf("AppendEvents returned error: %v", err)
	}
	events[0].Domain.Deal.HandSizes[0] = 99

	got := store.EventsForMatch(first)
	if len(got) != 1 {
		t.Fatalf("got %d events for match, want 1", len(got))
	}
	got[0].Domain.Deal.HandSizes[0] = 88

	again := store.EventsForMatch(first)
	if again[0].Domain.Deal.HandSizes[0] != 1 {
		t.Fatalf("stored hand sizes = %v, want copied [1 1]", again[0].Domain.Deal.HandSizes)
	}
}
