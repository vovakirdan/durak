package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

// ErrMissingConfigSnapshot means durable storage cannot identify match rules.
var ErrMissingConfigSnapshot = errors.New("missing config snapshot")

// MatchConfigSnapshot records the immutable config used to create a match.
type MatchConfigSnapshot struct {
	Identity MatchConfigIdentity
	Config   MatchConfig
}

// NewMatchConfigSnapshot creates a validated config snapshot.
func NewMatchConfigSnapshot(config *MatchConfig) (MatchConfigSnapshot, error) {
	if config == nil {
		return MatchConfigSnapshot{}, fmt.Errorf("%w: config is nil", ErrInvalidMatchConfig)
	}
	identity, err := config.Identity()
	if err != nil {
		return MatchConfigSnapshot{}, err
	}
	return MatchConfigSnapshot{Identity: identity, Config: *config}, nil
}

// ConfigJSON returns the stable JSON payload stored for this config snapshot.
func (s *MatchConfigSnapshot) ConfigJSON() ([]byte, error) {
	if s == nil {
		return nil, ErrMissingConfigSnapshot
	}
	data, err := json.Marshal(s.Config)
	if err != nil {
		return nil, fmt.Errorf("%w: marshal config snapshot: %w", ErrInvalidMatchConfig, err)
	}
	return data, nil
}

// MatchRecordBatch is one transactional persistence unit for a match stream.
type MatchRecordBatch struct {
	MatchID        MatchID
	SeriesID       SeriesID
	ConfigSnapshot *MatchConfigSnapshot
	PublicEvents   []Event
	InternalEvents []InternalEvent
	Summary        *MatchSummary
}

// MatchRecorder stores public/internal match data as one durable operation.
type MatchRecorder interface {
	RecordMatchBatch(context.Context, *MatchRecordBatch) error
}

func cloneConfigSnapshot(snapshot *MatchConfigSnapshot) *MatchConfigSnapshot {
	if snapshot == nil {
		return nil
	}
	cloned := *snapshot
	return &cloned
}
