package client

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const defaultMaxControllerTurns = 500

var (
	// ErrInvalidLocalGame means the local facade was configured incorrectly.
	ErrInvalidLocalGame = errors.New("invalid local game")
	// ErrUnknownActionID means the client submitted an action not present in state.
	ErrUnknownActionID = errors.New("unknown action id")
	// ErrMatchInProgress means the client requested a new match before this one ended.
	ErrMatchInProgress = errors.New("match in progress")
	// ErrNoActiveController means Advance found no controller-owned playable seat.
	ErrNoActiveController = errors.New("no active controller")
)

// LocalGameOptions configures a local in-process game facade.
type LocalGameOptions struct {
	SeriesID           app.SeriesID
	BaseMatchID        app.MatchID
	PlayerCount        int
	HumanSeat          domain.Seat
	Config             app.MatchConfig
	Deal               domain.DealOptions
	Controllers        map[domain.Seat]app.PlayerController
	MaxControllerTurns int
}

// LocalGame is a small in-process client contract for CLI/TUI development.
type LocalGame struct {
	series             *app.Series
	session            *app.Session
	controllers        map[domain.Seat]app.PlayerController
	humanSeat          domain.Seat
	baseMatchID        app.MatchID
	matchID            app.MatchID
	matchNumber        int
	turnNumber         int
	version            uint64
	deal               domain.DealOptions
	maxControllerTurns int
}

// NewLocalGame starts the first match for one local human and controller seats.
func NewLocalGame(ctx context.Context, options *LocalGameOptions) (*LocalGame, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options == nil {
		return nil, fmt.Errorf("%w: options are nil", ErrInvalidLocalGame)
	}
	config, playerCount, err := localGameConfig(options)
	if err != nil {
		return nil, err
	}
	if options.HumanSeat < 0 || int(options.HumanSeat) >= playerCount {
		return nil, fmt.Errorf("%w: human seat %d outside 0..%d", ErrInvalidLocalGame, options.HumanSeat, playerCount-1)
	}
	seriesID := options.SeriesID
	if seriesID == "" {
		seriesID = "local-series"
	}
	baseMatchID := options.BaseMatchID
	if baseMatchID == "" {
		baseMatchID = "local-match"
	}
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: seriesID,
		Seats:    canonicalSeats(playerCount),
		Config:   config,
	})
	if err != nil {
		return nil, err
	}
	controllers := maps.Clone(options.Controllers)
	if err := validateControllers(series.Seats(), options.HumanSeat, controllers); err != nil {
		return nil, err
	}
	maxControllerTurns := options.MaxControllerTurns
	if maxControllerTurns <= 0 {
		maxControllerTurns = defaultMaxControllerTurns
	}
	game := &LocalGame{
		series:             series,
		controllers:        controllers,
		humanSeat:          options.HumanSeat,
		baseMatchID:        baseMatchID,
		deal:               options.Deal,
		maxControllerTurns: maxControllerTurns,
	}
	if err := game.startNextMatch(ctx); err != nil {
		return nil, err
	}
	return game, nil
}

// State returns the current state for the local human seat.
func (g *LocalGame) State() State {
	if g == nil || g.session == nil {
		return State{}
	}
	decision := g.session.DecisionContext(g.humanSeat)
	return StateFromDecision(g.matchID, g.version, &decision)
}

// SubmitAction applies a legal action ID from the current human state.
func (g *LocalGame) SubmitAction(ctx context.Context, actionID string) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if g == nil || g.session == nil {
		return State{}, fmt.Errorf("%w: nil local game", ErrInvalidLocalGame)
	}
	decision := g.session.DecisionContext(g.humanSeat)
	index, err := strconv.Atoi(actionID)
	if err != nil || index < 1 || index > len(decision.LegalActions) {
		return g.State(), fmt.Errorf("%w: %q", ErrUnknownActionID, actionID)
	}
	if err := g.session.ApplyAction(ctx, decision.LegalActions[index-1]); err != nil {
		return g.State(), err
	}
	g.version++
	return g.State(), nil
}

// Advance runs controller turns until the human can act or the match completes.
func (g *LocalGame) Advance(ctx context.Context) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if g == nil || g.session == nil {
		return State{}, fmt.Errorf("%w: nil local game", ErrInvalidLocalGame)
	}
	for range g.maxControllerTurns {
		state := g.State()
		if state.Phase == "complete" || len(state.LegalActions) > 0 {
			return state, nil
		}
		seat := g.activeControllerSeat()
		if seat == domain.NoSeat {
			return state, ErrNoActiveController
		}
		controller := g.controllers[seat]
		if controller == nil {
			return state, fmt.Errorf("%w: seat %d", app.ErrMissingPlayerController, seat)
		}
		g.turnNumber++
		turn := g.turnContext(seat)
		controllerTurn := turn.Clone()
		decision, err := controller.Decide(ctx, &controllerTurn)
		if err != nil {
			return g.State(), err
		}
		if err := g.session.ApplyPlayerDecision(ctx, seat, &turn, decision); err != nil {
			return g.State(), err
		}
		g.version++
	}
	return g.State(), fmt.Errorf("%w: after %d controller turns", app.ErrActionLimitExceeded, g.maxControllerTurns)
}

// Concede gives up the current match for the local human seat.
func (g *LocalGame) Concede(ctx context.Context) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if g == nil || g.session == nil {
		return State{}, fmt.Errorf("%w: nil local game", ErrInvalidLocalGame)
	}
	if err := g.session.Concede(ctx, g.humanSeat); err != nil {
		return g.State(), err
	}
	g.version++
	return g.State(), nil
}

// NextMatch records the completed match and starts the next one in the series.
func (g *LocalGame) NextMatch(ctx context.Context) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if g == nil || g.session == nil {
		return State{}, fmt.Errorf("%w: nil local game", ErrInvalidLocalGame)
	}
	if g.State().Phase != "complete" {
		return g.State(), ErrMatchInProgress
	}
	if err := g.series.CompleteMatch(g.session); err != nil {
		return g.State(), err
	}
	if err := g.startNextMatch(ctx); err != nil {
		return State{}, err
	}
	g.version++
	return g.State(), nil
}

func localGameConfig(options *LocalGameOptions) (app.MatchConfig, int, error) {
	config := options.Config
	playerCount := options.PlayerCount
	if config == (app.MatchConfig{}) {
		if playerCount == 0 {
			playerCount = 2
		}
		var err error
		config, err = app.NewMatchConfig(app.RulePresetDefault, playerCount)
		if err != nil {
			return app.MatchConfig{}, 0, err
		}
		return config, playerCount, nil
	}
	if playerCount == 0 {
		playerCount = config.Seats.PlayerCount
	}
	if config.Seats.PlayerCount != playerCount {
		return app.MatchConfig{}, 0,
			fmt.Errorf("%w: config seats %d do not match player count %d",
				ErrInvalidLocalGame, config.Seats.PlayerCount, playerCount)
	}
	return config, playerCount, nil
}

func validateControllers(seats []domain.Seat, humanSeat domain.Seat, controllers map[domain.Seat]app.PlayerController) error {
	for _, seat := range seats {
		if seat == humanSeat {
			continue
		}
		if controllers[seat] == nil {
			return fmt.Errorf("%w: seat %d", app.ErrMissingPlayerController, seat)
		}
	}
	return nil
}

func canonicalSeats(count int) []domain.Seat {
	seats := make([]domain.Seat, count)
	for seat := range seats {
		seats[seat] = domain.Seat(seat)
	}
	return seats
}

func (g *LocalGame) startNextMatch(ctx context.Context) error {
	g.matchNumber++
	g.turnNumber = 0
	matchID := localMatchID(g.baseMatchID, g.matchNumber)
	session, _, err := g.series.StartMatch(ctx, &app.SeriesMatchOptions{
		MatchID: matchID,
		Deal:    g.deal,
	})
	if err != nil {
		return err
	}
	g.session = session
	g.matchID = matchID
	return nil
}
