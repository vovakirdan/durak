package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/vovakirdan/durak/internal/app"
)

// EventsForMatch reads public events for one match stream.
func (s *SQLiteStore) EventsForMatch(ctx context.Context, matchID app.MatchID) ([]app.Event, error) {
	var rows []publicEventRow
	if err := s.db.NewSelect().
		Model(&rows).
		Where("? = ?", bun.Ident("match_id"), string(matchID)).
		OrderExpr("? ASC", bun.Ident("sequence")).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("%w: read public events: %w", ErrInvalidSQLiteStore, err)
	}
	events := make([]app.Event, len(rows))
	for i := range rows {
		event, err := app.UnmarshalEventJSON([]byte(rows[i].EnvelopeJSON))
		if err != nil {
			return nil, fmt.Errorf("%w: decode public event %d: %w", ErrInvalidSQLiteStore, rows[i].Sequence, err)
		}
		events[i] = event
	}
	return events, nil
}

// InternalEventsForMatch reads canonical internal events for one match stream.
func (s *SQLiteStore) InternalEventsForMatch(ctx context.Context, matchID app.MatchID) ([]app.InternalEvent, error) {
	var rows []internalEventRow
	if err := s.db.NewSelect().
		Model(&rows).
		Where("? = ?", bun.Ident("match_id"), string(matchID)).
		OrderExpr("? ASC", bun.Ident("sequence")).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("%w: read internal events: %w", ErrInvalidSQLiteStore, err)
	}
	events := make([]app.InternalEvent, len(rows))
	for i := range rows {
		event, err := app.UnmarshalInternalEventJSON([]byte(rows[i].EnvelopeJSON))
		if err != nil {
			return nil, fmt.Errorf("%w: decode internal event %d: %w", ErrInvalidSQLiteStore, rows[i].Sequence, err)
		}
		events[i] = event
	}
	return events, nil
}

// MatchSummaries lists completed match summaries in projection order.
func (s *SQLiteStore) MatchSummaries(ctx context.Context) ([]app.MatchSummary, error) {
	var rows []summaryRow
	if err := s.db.NewSelect().
		Model(&rows).
		OrderExpr("? ASC", bun.Ident("projected_at")).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("%w: read summaries: %w", ErrInvalidSQLiteStore, err)
	}
	summaries := make([]app.MatchSummary, len(rows))
	for i := range rows {
		summary, err := rowSummary(&rows[i])
		if err != nil {
			return nil, err
		}
		snapshot, err := s.ConfigSnapshot(ctx, rows[i].ConfigHash)
		if err != nil {
			return nil, err
		}
		summary.ConfigIdentity = snapshot.Identity
		summaries[i] = summary
	}
	return summaries, nil
}

// ConfigSnapshot reads one stored config snapshot by hash.
func (s *SQLiteStore) ConfigSnapshot(ctx context.Context, hash string) (app.MatchConfigSnapshot, error) {
	row := configSnapshotRow{Hash: hash}
	if err := s.db.NewSelect().Model(&row).WherePK().Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return app.MatchConfigSnapshot{}, fmt.Errorf("%w: config snapshot %q not found", ErrInvalidSQLiteStore, hash)
		}
		return app.MatchConfigSnapshot{}, fmt.Errorf("%w: read config snapshot: %w", ErrInvalidSQLiteStore, err)
	}
	var config app.MatchConfig
	if err := json.Unmarshal([]byte(row.ConfigJSON), &config); err != nil {
		return app.MatchConfigSnapshot{}, fmt.Errorf("%w: decode config snapshot: %w", ErrInvalidSQLiteStore, err)
	}
	return app.MatchConfigSnapshot{
		Identity: app.MatchConfigIdentity{
			SchemaVersion: row.SchemaVersion,
			RulePreset:    row.RulePreset,
			RuleProfile:   row.RuleProfile,
			Hash:          row.Hash,
		},
		Config: config,
	}, nil
}
