package storage

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	sqliteStatusActive    = "active"
	sqliteStatusCompleted = "completed"
)

var (
	// ErrInvalidSQLiteStore means the SQLite store configuration or data is invalid.
	ErrInvalidSQLiteStore = errors.New("invalid sqlite store")
	// ErrSQLiteConflict means persisted match data conflicts with existing rows.
	ErrSQLiteConflict = errors.New("sqlite storage conflict")
)

//go:embed migrations/*.sql
var sqliteMigrations embed.FS

// SQLiteStore stores indexed match history in SQLite.
type SQLiteStore struct {
	db *bun.DB
}

// OpenSQLiteStore opens a SQLite database file and applies embedded migrations.
func OpenSQLiteStore(ctx context.Context, path string) (*SQLiteStore, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: path is empty", ErrInvalidSQLiteStore)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve path %q: %w", ErrInvalidSQLiteStore, path, err)
	}
	sqldb, err := sql.Open(sqliteshim.ShimName, sqliteDSN(abs))
	if err != nil {
		return nil, fmt.Errorf("%w: open %q: %w", ErrInvalidSQLiteStore, path, err)
	}
	sqldb.SetMaxOpenConns(1)

	store := &SQLiteStore{db: bun.NewDB(sqldb, sqlitedialect.New())}
	if err := store.Migrate(ctx); err != nil {
		closeErr := store.Close()
		if closeErr != nil {
			return nil, errors.Join(err, closeErr)
		}
		return nil, err
	}
	return store, nil
}

// Close closes the underlying database.
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// Migrate applies embedded Goose migrations.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.db == nil {
		return fmt.Errorf("%w: store is nil", ErrInvalidSQLiteStore)
	}
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(sqliteMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("%w: set migration dialect: %w", ErrInvalidSQLiteStore, err)
	}
	if err := goose.Up(s.db.DB, "migrations"); err != nil {
		return fmt.Errorf("%w: run migrations: %w", ErrInvalidSQLiteStore, err)
	}
	return nil
}

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

func sqliteDSN(absPath string) string {
	return "file:" + filepath.ToSlash(absPath) + "?cache=shared&mode=rwc&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
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

func newSummaryRow(summary *app.MatchSummary, projectedAt time.Time) (summaryRow, error) {
	if summary.ConfigIdentity.Hash == "" {
		return summaryRow{}, fmt.Errorf("%w: summary config hash is empty", ErrInvalidSQLiteStore)
	}
	seatsJSON, err := json.Marshal(summary.Seats)
	if err != nil {
		return summaryRow{}, err
	}
	handSizesJSON, err := json.Marshal(summary.InitialHandSizes)
	if err != nil {
		return summaryRow{}, err
	}
	return summaryRow{
		MatchID:              string(summary.MatchID),
		RuleProfile:          summary.RuleProfile,
		ConfigHash:           summary.ConfigIdentity.Hash,
		SeatsJSON:            string(seatsJSON),
		InitialHandSizesJSON: string(handSizesJSON),
		TrumpIndicatorRank:   int(summary.TrumpIndicator.Rank),
		TrumpIndicatorSuit:   int(summary.TrumpIndicator.Suit),
		TrumpSuit:            int(summary.TrumpSuit),
		FirstAttacker:        int(summary.FirstAttacker),
		InitialDefender:      int(summary.InitialDefender),
		InitialStockCount:    summary.InitialStockCount,
		ActionCount:          summary.ActionCount,
		LastSequence:         summary.LastSequence,
		Completed:            summary.Completed,
		Winner:               int(summary.Winner),
		Loser:                int(summary.Loser),
		Draw:                 summary.Draw,
		ConcededBy:           int(summary.ConcededBy),
		ProjectedAt:          projectedAt,
	}, nil
}

func rowSummary(row *summaryRow) (app.MatchSummary, error) {
	var seats []domain.Seat
	if err := json.Unmarshal([]byte(row.SeatsJSON), &seats); err != nil {
		return app.MatchSummary{}, fmt.Errorf("%w: decode summary seats: %w", ErrInvalidSQLiteStore, err)
	}
	var handSizes []int
	if err := json.Unmarshal([]byte(row.InitialHandSizesJSON), &handSizes); err != nil {
		return app.MatchSummary{}, fmt.Errorf("%w: decode summary hand sizes: %w", ErrInvalidSQLiteStore, err)
	}
	trumpIndicator, err := rowCard(row.TrumpIndicatorRank, row.TrumpIndicatorSuit)
	if err != nil {
		return app.MatchSummary{}, err
	}
	trumpSuit, err := rowSuit(row.TrumpSuit)
	if err != nil {
		return app.MatchSummary{}, err
	}
	return app.MatchSummary{
		MatchID: app.MatchID(row.MatchID),
		ConfigIdentity: app.MatchConfigIdentity{
			RuleProfile: row.RuleProfile,
			Hash:        row.ConfigHash,
		},
		RuleProfile:       row.RuleProfile,
		Seats:             seats,
		InitialHandSizes:  handSizes,
		TrumpIndicator:    trumpIndicator,
		TrumpSuit:         trumpSuit,
		FirstAttacker:     domain.Seat(row.FirstAttacker),
		InitialDefender:   domain.Seat(row.InitialDefender),
		InitialStockCount: row.InitialStockCount,
		ActionCount:       row.ActionCount,
		LastSequence:      row.LastSequence,
		Completed:         row.Completed,
		Winner:            domain.Seat(row.Winner),
		Loser:             domain.Seat(row.Loser),
		Draw:              row.Draw,
		ConcededBy:        domain.Seat(row.ConcededBy),
	}, nil
}

func rowCard(rankValue, suitValue int) (domain.Card, error) {
	rank, err := rowRank(rankValue)
	if err != nil {
		return domain.Card{}, err
	}
	suit, err := rowSuit(suitValue)
	if err != nil {
		return domain.Card{}, err
	}
	return domain.Card{Rank: rank, Suit: suit}, nil
}

func rowRank(value int) (domain.Rank, error) {
	if value < int(domain.Six) || value > int(domain.Ace) {
		return domain.RankUnknown, fmt.Errorf("%w: invalid rank %d", ErrInvalidSQLiteStore, value)
	}
	rank := domain.Rank(value)
	switch rank {
	case domain.Six, domain.Seven, domain.Eight, domain.Nine, domain.Ten,
		domain.Jack, domain.Queen, domain.King, domain.Ace:
		return rank, nil
	default:
		return domain.RankUnknown, fmt.Errorf("%w: invalid rank %d", ErrInvalidSQLiteStore, value)
	}
}

func rowSuit(value int) (domain.Suit, error) {
	if value < int(domain.Clubs) || value > int(domain.Spades) {
		return domain.SuitUnknown, fmt.Errorf("%w: invalid suit %d", ErrInvalidSQLiteStore, value)
	}
	suit := domain.Suit(value)
	switch suit {
	case domain.Clubs, domain.Diamonds, domain.Hearts, domain.Spades:
		return suit, nil
	default:
		return domain.SuitUnknown, fmt.Errorf("%w: invalid suit %d", ErrInvalidSQLiteStore, value)
	}
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
