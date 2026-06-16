package storage

import (
	"time"

	"github.com/uptrace/bun"
)

type configSnapshotRow struct {
	bun.BaseModel `bun:"table:match_config_snapshots"`

	Hash          string    `bun:",pk"`
	SchemaVersion int       `bun:"schema_version"`
	RulePreset    string    `bun:"rule_preset"`
	RuleProfile   string    `bun:"rule_profile"`
	ConfigJSON    string    `bun:"config_json"`
	CreatedAt     time.Time `bun:"created_at"`
}

type matchRow struct {
	bun.BaseModel `bun:"table:matches"`

	MatchID      string     `bun:"match_id,pk"`
	SeriesID     string     `bun:"series_id,nullzero"`
	ConfigHash   string     `bun:"config_hash"`
	PlayerCount  int        `bun:"player_count"`
	Status       string     `bun:"status"`
	StartedAt    time.Time  `bun:"started_at"`
	CompletedAt  *time.Time `bun:"completed_at,nullzero"`
	LastSequence uint64     `bun:"last_sequence"`
}

type internalEventRow struct {
	bun.BaseModel `bun:"table:match_internal_events"`

	MatchID               string    `bun:"match_id,pk"`
	Sequence              uint64    `bun:"sequence,pk"`
	Kind                  string    `bun:"kind"`
	EnvelopeSchemaVersion int       `bun:"envelope_schema_version"`
	EnvelopeJSON          string    `bun:"envelope_json"`
	CreatedAt             time.Time `bun:"created_at"`
}

type publicEventRow struct {
	bun.BaseModel `bun:"table:match_public_events"`

	MatchID               string    `bun:"match_id,pk"`
	Sequence              uint64    `bun:"sequence,pk"`
	Kind                  string    `bun:"kind"`
	EnvelopeSchemaVersion int       `bun:"envelope_schema_version"`
	EnvelopeJSON          string    `bun:"envelope_json"`
	CreatedAt             time.Time `bun:"created_at"`
}

type summaryRow struct {
	bun.BaseModel `bun:"table:match_summaries"`

	MatchID              string    `bun:"match_id,pk"`
	RuleProfile          string    `bun:"rule_profile"`
	ConfigHash           string    `bun:"config_hash"`
	SeatsJSON            string    `bun:"seats_json"`
	InitialHandSizesJSON string    `bun:"initial_hand_sizes_json"`
	TrumpIndicatorRank   int       `bun:"trump_indicator_rank"`
	TrumpIndicatorSuit   int       `bun:"trump_indicator_suit"`
	TrumpSuit            int       `bun:"trump_suit"`
	FirstAttacker        int       `bun:"first_attacker"`
	InitialDefender      int       `bun:"initial_defender"`
	InitialStockCount    int       `bun:"initial_stock_count"`
	ActionCount          int       `bun:"action_count"`
	LastSequence         uint64    `bun:"last_sequence"`
	Completed            bool      `bun:"completed"`
	Winner               int       `bun:"winner"`
	Loser                int       `bun:"loser"`
	Draw                 bool      `bun:"draw"`
	ConcededBy           int       `bun:"conceded_by"`
	ProjectedAt          time.Time `bun:"projected_at"`
}
