package app

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const matchConfigIdentityHashPrefix = "sha256:"

// MatchConfigIdentity identifies the immutable config used to start a match.
type MatchConfigIdentity struct {
	SchemaVersion int
	RulePreset    string
	RuleProfile   string
	Hash          string
}

// IsZero reports whether the identity is absent, preserving old event streams.
func (i MatchConfigIdentity) IsZero() bool {
	return i == MatchConfigIdentity{}
}

// Identity returns a stable, compact identity for this match config.
func (c *MatchConfig) Identity() (MatchConfigIdentity, error) {
	if err := c.Validate(); err != nil {
		return MatchConfigIdentity{}, err
	}
	profile, err := c.RuleProfile()
	if err != nil {
		return MatchConfigIdentity{}, err
	}
	data, err := json.Marshal(matchConfigIdentityHashPayload{
		SchemaVersion: c.SchemaVersion,
		RulePreset:    c.RulePreset,
		Seats:         c.Seats,
		Series:        c.Series,
		Rules:         c.Rules,
	})
	if err != nil {
		return MatchConfigIdentity{}, fmt.Errorf("%w: marshal config identity: %w", ErrInvalidMatchConfig, err)
	}
	sum := sha256.Sum256(data)
	return MatchConfigIdentity{
		SchemaVersion: c.SchemaVersion,
		RulePreset:    c.RulePreset,
		RuleProfile:   profile.Name,
		Hash:          matchConfigIdentityHashPrefix + hex.EncodeToString(sum[:]),
	}, nil
}

type matchConfigIdentityHashPayload struct {
	SchemaVersion int
	RulePreset    string
	Seats         SeatConfig
	Series        SeriesConfig
	Rules         RuleConfig
}
