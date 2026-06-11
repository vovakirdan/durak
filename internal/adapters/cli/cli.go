// Package cli contains the local command-line adapter for the first playable
// Durak interface.
package cli

import (
	"context"
	"errors"
	"fmt"
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
	SeriesID    app.SeriesID
	MatchID     app.MatchID
	EventStore  app.EventStore
}

// RunWithOptions starts the local CLI adapter.
func RunWithOptions(ctx context.Context, in io.Reader, out io.Writer, options *RunOptions) error {
	runOptions := normalizeRunOptions(options)
	if runOptions.Strategy == nil {
		return ErrMissingStrategy
	}
	if runOptions.EventStore != nil && runOptions.MatchID == "" {
		return app.ErrEmptyMatchID
	}

	runner, err := newSeriesRunner(&runOptions)
	if err != nil {
		return err
	}
	session, err := runner.startMatch(ctx)
	if err != nil {
		return err
	}

	game := newGame(session, runOptions.Strategy, in, out, gameOptions{
		humanSeat: defaultHumanSeat,
		botSeat:   defaultBotSeat,
		startNext: runner.startMatch,
		complete:  runner.completeMatch,
	})
	return game.run(ctx)
}

type seriesRunner struct {
	series      *app.Series
	deal        domain.DealOptions
	eventStore  app.EventStore
	baseMatchID app.MatchID
	matchNumber int
}

func newSeriesRunner(options *RunOptions) (*seriesRunner, error) {
	if options.PlayerCount != defaultPlayerCount {
		return nil, fmt.Errorf("%w: CLI currently supports exactly %d players", app.ErrInvalidSeries, defaultPlayerCount)
	}
	seriesID := options.SeriesID
	if seriesID == "" {
		seriesID = "cli-series"
	}
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: seriesID,
		Seats:    []domain.Seat{defaultHumanSeat, defaultBotSeat},
		Profile:  options.Profile,
	})
	if err != nil {
		return nil, err
	}
	return &seriesRunner{
		series:      series,
		deal:        options.Deal,
		eventStore:  options.EventStore,
		baseMatchID: options.MatchID,
	}, nil
}

func (r *seriesRunner) startMatch(ctx context.Context) (*app.Session, error) {
	r.matchNumber++
	session, _, err := r.series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID:    matchIDFor(r.baseMatchID, r.matchNumber),
		Deal:       r.deal,
		EventStore: r.eventStore,
	})
	if err != nil {
		r.matchNumber--
		return nil, err
	}
	return session, nil
}

func (r *seriesRunner) completeMatch(session *app.Session) error {
	return r.series.CompleteMatch(session)
}

func matchIDFor(base app.MatchID, matchNumber int) app.MatchID {
	if base == "" {
		return app.MatchID(fmt.Sprintf("cli-match-%d", matchNumber))
	}
	if matchNumber == 1 {
		return base
	}
	return app.MatchID(fmt.Sprintf("%s-%d", base, matchNumber))
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
