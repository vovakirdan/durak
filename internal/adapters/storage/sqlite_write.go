package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/vovakirdan/durak/internal/app"
)

// RecordMatchBatch stores one app persistence batch transactionally.
func (s *SQLiteStore) RecordMatchBatch(ctx context.Context, batch *app.MatchRecordBatch) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidSQLiteStore)
	}
	if err := validateRecordBatch(batch); err != nil {
		return err
	}
	return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		now := time.Now().UTC()
		match, err := s.matchForUpdate(ctx, tx, batch, now)
		if err != nil {
			return err
		}
		if err := s.insertInternalEvents(ctx, tx, batch.InternalEvents, now); err != nil {
			return err
		}
		if err := s.insertPublicEvents(ctx, tx, batch.PublicEvents, now); err != nil {
			return err
		}
		if batch.Summary != nil {
			if err := s.insertSummary(ctx, tx, batch.Summary, now); err != nil {
				return err
			}
			completedAt := now
			match.Status = sqliteStatusCompleted
			match.CompletedAt = &completedAt
		}
		match.LastSequence = maxSequence(match.LastSequence, batch.PublicEvents, batch.InternalEvents)
		return s.updateMatch(ctx, tx, match)
	})
}

func validateRecordBatch(batch *app.MatchRecordBatch) error {
	if batch == nil {
		return fmt.Errorf("%w: batch is nil", ErrInvalidSQLiteStore)
	}
	if batch.MatchID == "" {
		return fmt.Errorf("%w: match id is empty", ErrInvalidSQLiteStore)
	}
	if len(batch.PublicEvents) == 0 && len(batch.InternalEvents) == 0 && batch.Summary == nil {
		return fmt.Errorf("%w: batch is empty", ErrInvalidSQLiteStore)
	}
	for i := range batch.PublicEvents {
		if batch.PublicEvents[i].MatchID != batch.MatchID {
			return fmt.Errorf("%w: public event match id %q, want %q",
				ErrInvalidSQLiteStore, batch.PublicEvents[i].MatchID, batch.MatchID)
		}
	}
	for i := range batch.InternalEvents {
		if batch.InternalEvents[i].MatchID != batch.MatchID {
			return fmt.Errorf("%w: internal event match id %q, want %q",
				ErrInvalidSQLiteStore, batch.InternalEvents[i].MatchID, batch.MatchID)
		}
	}
	if batch.Summary != nil && batch.Summary.MatchID != batch.MatchID {
		return fmt.Errorf("%w: summary match id %q, want %q",
			ErrInvalidSQLiteStore, batch.Summary.MatchID, batch.MatchID)
	}
	return nil
}

func (s *SQLiteStore) matchForUpdate(
	ctx context.Context,
	tx bun.Tx,
	batch *app.MatchRecordBatch,
	now time.Time,
) (*matchRow, error) {
	if batch.ConfigSnapshot != nil {
		if err := s.ensureConfigSnapshot(ctx, tx, batch.ConfigSnapshot, now); err != nil {
			return nil, err
		}
	}
	match := matchRow{MatchID: string(batch.MatchID)}
	err := tx.NewSelect().Model(&match).WherePK().Scan(ctx)
	if err == nil {
		return &match, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: read match: %w", ErrInvalidSQLiteStore, err)
	}
	if batch.ConfigSnapshot == nil {
		return nil, fmt.Errorf("%w: new match needs config snapshot", app.ErrMissingConfigSnapshot)
	}
	match = matchRow{
		MatchID:      string(batch.MatchID),
		SeriesID:     string(batch.SeriesID),
		ConfigHash:   batch.ConfigSnapshot.Identity.Hash,
		PlayerCount:  playerCount(batch),
		Status:       sqliteStatusActive,
		StartedAt:    now,
		LastSequence: 0,
	}
	if _, err := tx.NewInsert().Model(&match).Exec(ctx); err != nil {
		return nil, fmt.Errorf("%w: insert match: %w", ErrSQLiteConflict, err)
	}
	return &match, nil
}

func (s *SQLiteStore) ensureConfigSnapshot(
	ctx context.Context,
	tx bun.Tx,
	snapshot *app.MatchConfigSnapshot,
	now time.Time,
) error {
	row := configSnapshotRow{Hash: snapshot.Identity.Hash}
	err := tx.NewSelect().Model(&row).WherePK().Scan(ctx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: read config snapshot: %w", ErrInvalidSQLiteStore, err)
	}
	configJSON, err := snapshot.ConfigJSON()
	if err != nil {
		return err
	}
	row = configSnapshotRow{
		Hash:          snapshot.Identity.Hash,
		SchemaVersion: snapshot.Identity.SchemaVersion,
		RulePreset:    snapshot.Identity.RulePreset,
		RuleProfile:   snapshot.Identity.RuleProfile,
		ConfigJSON:    string(configJSON),
		CreatedAt:     now,
	}
	if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
		return fmt.Errorf("%w: insert config snapshot: %w", ErrSQLiteConflict, err)
	}
	return nil
}

func (s *SQLiteStore) insertInternalEvents(
	ctx context.Context,
	tx bun.Tx,
	events []app.InternalEvent,
	now time.Time,
) error {
	for i := range events {
		envelope, err := app.NewInternalEventEnvelope(&events[i])
		if err != nil {
			return err
		}
		data, err := app.MarshalInternalEventJSON(&events[i])
		if err != nil {
			return err
		}
		row := internalEventRow{
			MatchID:               string(events[i].MatchID),
			Sequence:              events[i].Sequence,
			Kind:                  envelope.Kind,
			EnvelopeSchemaVersion: envelope.SchemaVersion,
			EnvelopeJSON:          string(data),
			CreatedAt:             now,
		}
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			return fmt.Errorf("%w: insert internal event %d: %w", ErrSQLiteConflict, events[i].Sequence, err)
		}
	}
	return nil
}

func (s *SQLiteStore) insertPublicEvents(
	ctx context.Context,
	tx bun.Tx,
	events []app.Event,
	now time.Time,
) error {
	for i := range events {
		envelope, err := app.NewEventEnvelope(&events[i])
		if err != nil {
			return err
		}
		data, err := app.MarshalEventJSON(&events[i])
		if err != nil {
			return err
		}
		row := publicEventRow{
			MatchID:               string(events[i].MatchID),
			Sequence:              events[i].Sequence,
			Kind:                  envelope.Kind,
			EnvelopeSchemaVersion: envelope.SchemaVersion,
			EnvelopeJSON:          string(data),
			CreatedAt:             now,
		}
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			return fmt.Errorf("%w: insert public event %d: %w", ErrSQLiteConflict, events[i].Sequence, err)
		}
	}
	return nil
}

func (s *SQLiteStore) insertSummary(ctx context.Context, tx bun.Tx, summary *app.MatchSummary, now time.Time) error {
	row, err := newSummaryRow(summary, now)
	if err != nil {
		return err
	}
	if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
		return fmt.Errorf("%w: insert summary: %w", ErrSQLiteConflict, err)
	}
	return nil
}

func (s *SQLiteStore) updateMatch(ctx context.Context, tx bun.Tx, match *matchRow) error {
	_, err := tx.NewUpdate().
		Model(match).
		WherePK().
		Column("status", "completed_at", "last_sequence").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("%w: update match: %w", ErrInvalidSQLiteStore, err)
	}
	return nil
}

func playerCount(batch *app.MatchRecordBatch) int {
	for i := range batch.PublicEvents {
		started := batch.PublicEvents[i].Domain.Started
		if started != nil {
			return started.PlayerCount
		}
	}
	if batch.Summary != nil {
		return len(batch.Summary.Seats)
	}
	return batch.ConfigSnapshot.Config.Seats.PlayerCount
}

func maxSequence(current uint64, publicEvents []app.Event, internalEvents []app.InternalEvent) uint64 {
	for i := range publicEvents {
		if publicEvents[i].Sequence > current {
			current = publicEvents[i].Sequence
		}
	}
	for i := range internalEvents {
		if internalEvents[i].Sequence > current {
			current = internalEvents[i].Sequence
		}
	}
	return current
}
