// Package cli contains the local command-line adapter for the first playable
// Durak interface.
package cli

import (
	"context"
	"errors"
	"io"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	defaultHumanSeat   = domain.Seat(0)
	defaultBotSeat     = domain.Seat(1)
	defaultPlayerCount = 2
)

// ErrMissingStrategy means CLI wiring did not provide a bot strategy.
var ErrMissingStrategy = errors.New("missing bot strategy")

// RunOptions configures the local CLI adapter without coupling it to concrete
// bot or deal implementations.
type RunOptions struct {
	PlayerCount int
	Profile     domain.RuleProfile
	Deal        domain.DealOptions
	Strategy    app.Strategy
	MatchID     app.MatchID
	EventStore  app.EventStore
}

// RunWithOptions starts the local CLI adapter.
func RunWithOptions(ctx context.Context, in io.Reader, out io.Writer, options *RunOptions) error {
	runOptions := normalizeRunOptions(options)
	if runOptions.Strategy == nil {
		return ErrMissingStrategy
	}

	session, _, err := app.NewDealtSessionWithOptions(ctx, runOptions.PlayerCount, runOptions.Profile, runOptions.Deal, app.SessionOptions{
		MatchID:    runOptions.MatchID,
		EventStore: runOptions.EventStore,
	})
	if err != nil {
		return err
	}

	game := newGame(session, runOptions.Strategy, in, out, gameOptions{
		humanSeat: defaultHumanSeat,
		botSeat:   defaultBotSeat,
	})
	return game.run(ctx)
}

func normalizeRunOptions(options *RunOptions) RunOptions {
	normalized := RunOptions{}
	if options != nil {
		normalized = *options
	}
	if normalized.PlayerCount == 0 {
		normalized.PlayerCount = defaultPlayerCount
	}
	if normalized.Profile == (domain.RuleProfile{}) {
		normalized.Profile = domain.DefaultRuleProfile()
	}
	return normalized
}
