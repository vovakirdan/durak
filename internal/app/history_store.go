package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/vovakirdan/durak/internal/domain"
)

// ErrNilHistoryStore means a history/replay operation has no backing store.
var ErrNilHistoryStore = errors.New("nil history store")

// MatchSummaryReader reads completed-match history projections.
type MatchSummaryReader interface {
	MatchSummaries(context.Context) ([]MatchSummary, error)
}

// MatchReplayReader reads durable match data needed for exact replay.
type MatchReplayReader interface {
	InternalEventsForMatch(context.Context, MatchID) ([]InternalEvent, error)
	ConfigSnapshot(context.Context, string) (MatchConfigSnapshot, error)
}

// ReadMatchSummaries reads completed match summaries from a durable history store.
func ReadMatchSummaries(ctx context.Context, reader MatchSummaryReader) ([]MatchSummary, error) {
	if reader == nil {
		return nil, ErrNilHistoryStore
	}
	return reader.MatchSummaries(ctx)
}

// ReplayMatchFromHistory reconstructs a match from stored internal events and config.
func ReplayMatchFromHistory(ctx context.Context, reader MatchReplayReader, matchID MatchID) (ReplayResult, error) {
	if reader == nil {
		return ReplayResult{}, ErrNilHistoryStore
	}
	events, err := reader.InternalEventsForMatch(ctx, matchID)
	if err != nil {
		return ReplayResult{}, err
	}
	profile, err := replayRuleProfile(ctx, reader, events)
	if err != nil {
		return ReplayResult{}, err
	}
	return ReplayInternalEvents(events, profile)
}

func replayRuleProfile(ctx context.Context, reader MatchReplayReader, events []InternalEvent) (domain.RuleProfile, error) {
	if len(events) == 0 || events[0].ConfigIdentity.Hash == "" {
		return domain.DefaultRuleProfile(), nil
	}
	snapshot, err := reader.ConfigSnapshot(ctx, events[0].ConfigIdentity.Hash)
	if err != nil {
		return domain.RuleProfile{}, err
	}
	config := snapshot.Config
	profile, err := config.RuleProfile()
	if err != nil {
		return domain.RuleProfile{}, fmt.Errorf("%w: replay config profile: %w", ErrInvalidReplay, err)
	}
	return profile, nil
}
