-- +goose Up
CREATE TABLE match_config_snapshots (
    hash TEXT PRIMARY KEY,
    schema_version INTEGER NOT NULL,
    rule_preset TEXT NOT NULL,
    rule_profile TEXT NOT NULL,
    config_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE matches (
    match_id TEXT PRIMARY KEY,
    series_id TEXT,
    config_hash TEXT NOT NULL REFERENCES match_config_snapshots(hash),
    player_count INTEGER NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP,
    last_sequence INTEGER NOT NULL
);

CREATE TABLE match_internal_events (
    match_id TEXT NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    kind TEXT NOT NULL,
    envelope_schema_version INTEGER NOT NULL,
    envelope_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (match_id, sequence)
);

CREATE TABLE match_public_events (
    match_id TEXT NOT NULL REFERENCES matches(match_id) ON DELETE CASCADE,
    sequence INTEGER NOT NULL,
    kind TEXT NOT NULL,
    envelope_schema_version INTEGER NOT NULL,
    envelope_json TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    PRIMARY KEY (match_id, sequence)
);

CREATE TABLE match_summaries (
    match_id TEXT PRIMARY KEY REFERENCES matches(match_id) ON DELETE CASCADE,
    rule_profile TEXT NOT NULL,
    config_hash TEXT NOT NULL REFERENCES match_config_snapshots(hash),
    seats_json TEXT NOT NULL,
    initial_hand_sizes_json TEXT NOT NULL,
    trump_indicator_rank INTEGER NOT NULL,
    trump_indicator_suit INTEGER NOT NULL,
    trump_suit INTEGER NOT NULL,
    first_attacker INTEGER NOT NULL,
    initial_defender INTEGER NOT NULL,
    initial_stock_count INTEGER NOT NULL,
    action_count INTEGER NOT NULL,
    last_sequence INTEGER NOT NULL,
    completed BOOLEAN NOT NULL,
    winner INTEGER NOT NULL,
    loser INTEGER NOT NULL,
    draw BOOLEAN NOT NULL,
    conceded_by INTEGER NOT NULL,
    projected_at TIMESTAMP NOT NULL
);

CREATE INDEX idx_matches_series_id ON matches(series_id);
CREATE INDEX idx_match_public_events_match_sequence ON match_public_events(match_id, sequence);
CREATE INDEX idx_match_internal_events_match_sequence ON match_internal_events(match_id, sequence);

-- +goose Down
DROP TABLE match_summaries;
DROP TABLE match_public_events;
DROP TABLE match_internal_events;
DROP TABLE matches;
DROP TABLE match_config_snapshots;
