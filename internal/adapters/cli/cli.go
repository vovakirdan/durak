// Package cli contains the local command-line adapter for the first playable
// Durak interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	defaultHumanSeat   = domain.Seat(0)
	defaultBotSeat     = domain.Seat(1)
	defaultPlayerCount = 2
)

var (
	// ErrMissingStrategy means CLI wiring did not provide a controller.
	ErrMissingStrategy = errors.New("missing bot controller")
	// ErrInvalidHumanSeat means the configured local human seat is outside the table.
	ErrInvalidHumanSeat = errors.New("invalid human seat")
)

// RunOptions configures the local CLI adapter without coupling it to concrete
// bot or deal implementations.
type RunOptions struct {
	PlayerCount int
	HumanSeat   domain.Seat
	Config      app.MatchConfig
	Deal        domain.DealOptions
	Strategy    app.Strategy
	Bot         app.PlayerController
	Controllers map[domain.Seat]app.PlayerController
	SeriesID    app.SeriesID
	MatchID     app.MatchID
	EventStore  app.EventStore
}

// RunWithOptions starts the local CLI adapter.
func RunWithOptions(ctx context.Context, in io.Reader, out io.Writer, options *RunOptions) error {
	runOptions, err := normalizeRunOptions(options)
	if err != nil {
		return err
	}
	if runOptions.EventStore != nil && runOptions.MatchID == "" {
		return app.ErrEmptyMatchID
	}
	controllers, err := runControllers(&runOptions)
	if err != nil {
		return err
	}

	runner, err := newSeriesRunner(&runOptions)
	if err != nil {
		return err
	}
	session, err := runner.startMatch(ctx)
	if err != nil {
		return err
	}

	game := newGameWithControllers(session, controllers, in, out, gameOptions{
		humanSeat: runOptions.HumanSeat,
		startNext: runner.startMatch,
		complete:  runner.completeMatch,
	})
	return game.run(ctx)
}

func runControllers(options *RunOptions) (map[domain.Seat]app.PlayerController, error) {
	controllers := maps.Clone(options.Controllers)
	if controllers == nil {
		controllers = make(map[domain.Seat]app.PlayerController, options.PlayerCount-1)
	}
	if len(controllers) == 0 {
		controller := options.Bot
		if controller == nil && options.Strategy != nil {
			controller = app.StrategyController{Strategy: options.Strategy}
		}
		if controller == nil {
			return nil, ErrMissingStrategy
		}
		for _, seat := range canonicalSeats(options.PlayerCount) {
			if seat != options.HumanSeat {
				controllers[seat] = controller
			}
		}
	}
	for _, seat := range canonicalSeats(options.PlayerCount) {
		if seat == options.HumanSeat {
			continue
		}
		if controllers[seat] == nil {
			return nil, fmt.Errorf("%w: seat %d", ErrMissingStrategy, seat)
		}
	}
	return controllers, nil
}

type seriesRunner struct {
	series      *app.Series
	deal        domain.DealOptions
	eventStore  app.EventStore
	baseMatchID app.MatchID
	matchNumber int
}

func newSeriesRunner(options *RunOptions) (*seriesRunner, error) {
	if options.HumanSeat < 0 || int(options.HumanSeat) >= options.PlayerCount {
		return nil, fmt.Errorf("%w: got %d, allowed 0..%d", ErrInvalidHumanSeat, options.HumanSeat, options.PlayerCount-1)
	}
	seriesID := options.SeriesID
	if seriesID == "" {
		seriesID = "cli-series"
	}
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: seriesID,
		Seats:    canonicalSeats(options.PlayerCount),
		Config:   options.Config,
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

func canonicalSeats(count int) []domain.Seat {
	seats := make([]domain.Seat, count)
	for seat := range seats {
		seats[seat] = domain.Seat(seat)
	}
	return seats
}

func (r *seriesRunner) startMatch(ctx context.Context) (*app.Session, error) {
	r.matchNumber++
	session, _, err := r.series.StartMatch(ctx, &app.SeriesMatchOptions{
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

func normalizeRunOptions(options *RunOptions) (RunOptions, error) {
	normalized := RunOptions{}
	if options != nil {
		normalized = *options
	}
	if normalized.PlayerCount == 0 {
		if normalized.Config != (app.MatchConfig{}) {
			normalized.PlayerCount = normalized.Config.Seats.PlayerCount
		} else {
			normalized.PlayerCount = defaultPlayerCount
		}
	}
	if normalized.Config == (app.MatchConfig{}) {
		config, err := app.NewMatchConfig(app.RulePresetDefault, normalized.PlayerCount)
		if err != nil {
			return RunOptions{}, err
		}
		normalized.Config = config
	}
	return normalized, nil
}
