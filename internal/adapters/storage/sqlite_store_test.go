package storage_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/storage"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSQLiteStoreRecordsAndReadsMatchData(t *testing.T) {
	ctx := context.Background()
	store := mustSQLiteStore(t)
	deal := sqliteReplayableDeal()
	snapshot := mustConfigSnapshot(t, 2)
	matchID := app.MatchID("sqlite-match")

	session, err := app.NewSessionWithOptions(ctx, mustSQLiteMatch(t, deal), &app.SessionOptions{
		MatchID:        matchID,
		SeriesID:       app.SeriesID("sqlite-series"),
		ConfigSnapshot: &snapshot,
		MatchRecorder:  store,
		InitialDeal:    &deal,
	})
	if err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}

	attack := deal.Hands[0][0]
	defense := deal.Hands[1][0]
	mustApplySQLiteAction(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApplySQLiteAction(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApplySQLiteAction(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	events, err := store.EventsForMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("EventsForMatch returned error: %v", err)
	}
	wantKinds := []domain.EventKind{
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindAttack,
		domain.EventKindDefend,
		domain.EventKindFinishDefense,
		domain.EventKindRoundEnded,
		domain.EventKindMatchEnded,
	}
	if gotKinds := eventKinds(events); !slices.Equal(gotKinds, wantKinds) {
		t.Fatalf("event kinds = %v, want %v", gotKinds, wantKinds)
	}

	internalEvents, err := store.InternalEventsForMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("InternalEventsForMatch returned error: %v", err)
	}
	if len(internalEvents) != len(events) {
		t.Fatalf("internal event count = %d, want %d", len(internalEvents), len(events))
	}
	if internalEvents[1].Deal == nil {
		t.Fatalf("internal deal payload is nil")
	}
	replay, err := app.ReplayInternalEvents(internalEvents, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("ReplayInternalEvents returned error: %v", err)
	}
	if !reflect.DeepEqual(replay.Events, domainEventsFromInternal(internalEvents)) {
		t.Fatalf("replayed events differ from stored internal stream")
	}

	gotSnapshot, err := store.ConfigSnapshot(ctx, snapshot.Identity.Hash)
	if err != nil {
		t.Fatalf("ConfigSnapshot returned error: %v", err)
	}
	if !reflect.DeepEqual(gotSnapshot, snapshot) {
		t.Fatalf("config snapshot = %+v, want %+v", gotSnapshot, snapshot)
	}

	summaries, err := store.MatchSummaries(ctx)
	if err != nil {
		t.Fatalf("MatchSummaries returned error: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("summary count = %d, want 1", len(summaries))
	}
	summary := summaries[0]
	if summary.MatchID != matchID || !summary.Completed || !summary.Draw {
		t.Fatalf("summary = %+v, want completed draw for %q", summary, matchID)
	}
	if summary.LastSequence != uint64(len(events)) {
		t.Fatalf("summary last sequence = %d, want %d", summary.LastSequence, len(events))
	}
	if !reflect.DeepEqual(summary.ConfigIdentity, snapshot.Identity) {
		t.Fatalf("summary config identity = %+v, want %+v", summary.ConfigIdentity, snapshot.Identity)
	}
}

func TestSQLiteStoreRollsBackBatchOnConflict(t *testing.T) {
	ctx := context.Background()
	store := mustSQLiteStore(t)
	deal := sqliteReplayableDeal()
	snapshot := mustConfigSnapshot(t, 2)
	matchID := app.MatchID("sqlite-conflict")

	if _, err := app.NewSessionWithOptions(ctx, mustSQLiteMatch(t, deal), &app.SessionOptions{
		MatchID:        matchID,
		ConfigSnapshot: &snapshot,
		MatchRecorder:  store,
		InitialDeal:    &deal,
	}); err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}

	err := store.RecordMatchBatch(ctx, &app.MatchRecordBatch{
		MatchID: matchID,
		PublicEvents: []app.Event{
			testStartedEvent(matchID, 1),
		},
		InternalEvents: []app.InternalEvent{
			{
				MatchID:  matchID,
				Sequence: 3,
				Domain:   testActionEvent(matchID, 3).Domain,
			},
		},
	})
	if !errors.Is(err, storage.ErrSQLiteConflict) {
		t.Fatalf("RecordMatchBatch error = %v, want ErrSQLiteConflict", err)
	}

	internalEvents, err := store.InternalEventsForMatch(ctx, matchID)
	if err != nil {
		t.Fatalf("InternalEventsForMatch returned error: %v", err)
	}
	if len(internalEvents) != 2 {
		t.Fatalf("internal event count after rolled-back conflict = %d, want 2", len(internalEvents))
	}
}

func TestSQLiteStoreRejectsNewMatchWithoutConfigSnapshot(t *testing.T) {
	ctx := context.Background()
	store := mustSQLiteStore(t)
	matchID := app.MatchID("sqlite-no-config")

	err := store.RecordMatchBatch(ctx, &app.MatchRecordBatch{
		MatchID:      matchID,
		PublicEvents: []app.Event{testStartedEvent(matchID, 1)},
	})
	if !errors.Is(err, app.ErrMissingConfigSnapshot) {
		t.Fatalf("RecordMatchBatch error = %v, want ErrMissingConfigSnapshot", err)
	}
}

func mustSQLiteStore(t *testing.T) *storage.SQLiteStore {
	t.Helper()
	store, err := storage.OpenSQLiteStore(context.Background(), filepath.Join(t.TempDir(), "durak.db"))
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})
	return store
}

func sqliteReplayableDeal() domain.InitialDeal {
	return domain.InitialDeal{
		Hands: [][]domain.Card{
			{{Rank: domain.Six, Suit: domain.Clubs}},
			{{Rank: domain.Seven, Suit: domain.Clubs}},
		},
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}
}

func mustConfigSnapshot(t *testing.T, playerCount int) app.MatchConfigSnapshot {
	t.Helper()
	config, err := app.NewMatchConfig(app.RulePresetDefault, playerCount)
	if err != nil {
		t.Fatalf("NewMatchConfig returned error: %v", err)
	}
	snapshot, err := app.NewMatchConfigSnapshot(&config)
	if err != nil {
		t.Fatalf("NewMatchConfigSnapshot returned error: %v", err)
	}
	return snapshot
}

func mustSQLiteMatch(t *testing.T, deal domain.InitialDeal) *domain.Match {
	t.Helper()
	match, err := domain.NewMatch(&deal, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	return match
}

func mustApplySQLiteAction(t *testing.T, session *app.Session, action domain.Action) {
	t.Helper()
	if err := session.ApplyAction(context.Background(), action); err != nil {
		t.Fatalf("ApplyAction(%+v) returned error: %v", action, err)
	}
}

func eventKinds(events []app.Event) []domain.EventKind {
	kinds := make([]domain.EventKind, len(events))
	for i := range events {
		kinds[i] = events[i].Domain.Kind
	}
	return kinds
}

func domainEventsFromInternal(events []app.InternalEvent) []domain.Event {
	domainEvents := make([]domain.Event, len(events))
	for i := range events {
		domainEvents[i] = events[i].Domain.Clone()
	}
	return domainEvents
}
