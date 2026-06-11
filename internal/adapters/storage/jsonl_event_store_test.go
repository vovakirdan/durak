package storage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestJSONLEventStoreAppendsAndReadsEvents(t *testing.T) {
	ctx := context.Background()
	store := mustJSONLStore(t)
	events := []app.Event{
		testStartedEvent("match-1", 1),
		testActionEvent("match-1", 2),
		testStartedEvent("match-2", 1),
	}

	if err := store.AppendEvents(ctx, events[:2]); err != nil {
		t.Fatalf("AppendEvents first batch returned error: %v", err)
	}
	if err := store.AppendEvents(ctx, events[2:]); err != nil {
		t.Fatalf("AppendEvents second batch returned error: %v", err)
	}

	got, err := store.Events(ctx)
	if err != nil {
		t.Fatalf("Events returned error: %v", err)
	}
	if !reflect.DeepEqual(got, events) {
		t.Fatalf("events = %+v, want %+v", got, events)
	}

	matchEvents, err := store.EventsForMatch(ctx, app.MatchID("match-1"))
	if err != nil {
		t.Fatalf("EventsForMatch returned error: %v", err)
	}
	if !reflect.DeepEqual(matchEvents, events[:2]) {
		t.Fatalf("match events = %+v, want %+v", matchEvents, events[:2])
	}
}

func TestJSONLEventStoreRejectsInvalidBatchBeforeWriting(t *testing.T) {
	ctx := context.Background()
	store := mustJSONLStore(t)
	validEvent := testStartedEvent("match-1", 1)
	if err := store.AppendEvents(ctx, []app.Event{validEvent}); err != nil {
		t.Fatalf("AppendEvents valid event returned error: %v", err)
	}
	before := readFile(t, store.Path())

	err := store.AppendEvents(ctx, []app.Event{
		testStartedEvent("match-1", 2),
		{Sequence: 3, Domain: domain.Event{Kind: domain.EventKindMatchStarted}},
	})
	if !errors.Is(err, app.ErrInvalidEventEnvelope) {
		t.Fatalf("AppendEvents error = %v, want ErrInvalidEventEnvelope", err)
	}
	if after := readFile(t, store.Path()); after != before {
		t.Fatalf("file changed after invalid batch:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestJSONLEventStoreReturnsCorruptEventLog(t *testing.T) {
	ctx := context.Background()
	store := mustJSONLStore(t)
	event := testStartedEvent("match-1", 1)
	data, err := app.MarshalEventJSON(&event)
	if err != nil {
		t.Fatalf("MarshalEventJSON returned error: %v", err)
	}
	content := string(data) + "\n" + `{"schema_version":1,"match_id":"match-1"` + "\n"
	if writeErr := os.WriteFile(store.Path(), []byte(content), 0o600); writeErr != nil {
		t.Fatalf("WriteFile returned error: %v", writeErr)
	}

	_, err = store.Events(ctx)
	if !errors.Is(err, storage.ErrCorruptEventLog) {
		t.Fatalf("Events error = %v, want ErrCorruptEventLog", err)
	}
	if !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("Events error = %v, want line number", err)
	}
}

func TestNewJSONLEventStoreRejectsEmptyPath(t *testing.T) {
	_, err := storage.NewJSONLEventStore("")
	if !errors.Is(err, storage.ErrInvalidEventLog) {
		t.Fatalf("NewJSONLEventStore error = %v, want ErrInvalidEventLog", err)
	}
}

func mustJSONLStore(t *testing.T) *storage.JSONLEventStore {
	t.Helper()
	store, err := storage.NewJSONLEventStore(filepath.Join(t.TempDir(), "events.jsonl"))
	if err != nil {
		t.Fatalf("NewJSONLEventStore returned error: %v", err)
	}
	return store
}

func testStartedEvent(matchID app.MatchID, sequence uint64) app.Event {
	return app.Event{
		MatchID:  matchID,
		Sequence: sequence,
		Domain: domain.Event{
			Kind:    domain.EventKindMatchStarted,
			Started: &domain.MatchStartedEvent{PlayerCount: 2, RuleProfile: "classic"},
		},
	}
}

func testActionEvent(matchID app.MatchID, sequence uint64) app.Event {
	return app.Event{
		MatchID:  matchID,
		Sequence: sequence,
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
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	return string(data)
}
