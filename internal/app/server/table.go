package server

import (
	"context"
	"fmt"
	"maps"
	"strconv"
	"sync"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

const defaultMaxTableControllerTurns = 500

// TableOptions configures one in-memory daemon table.
type TableOptions struct {
	SeriesID           app.SeriesID
	BaseMatchID        app.MatchID
	PlayerCount        int
	HumanSeats         []domain.Seat
	Config             app.MatchConfig
	Deal               domain.DealOptions
	Controllers        map[domain.Seat]app.PlayerController
	MaxControllerTurns int
}

type table struct {
	mu                 sync.Mutex
	series             *app.Series
	session            *app.Session
	controllers        map[domain.Seat]app.PlayerController
	humanSeats         map[domain.Seat]bool
	humanSeatOrder     []domain.Seat
	occupiedSeats      map[domain.Seat]bool
	baseMatchID        app.MatchID
	matchID            app.MatchID
	completedMatchID   app.MatchID
	matchNumber        int
	turnNumber         int
	version            uint64
	deal               domain.DealOptions
	maxControllerTurns int
}

func newTable(ctx context.Context, options *TableOptions) (*table, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if options == nil {
		return nil, fmt.Errorf("%w: options are nil", ErrInvalidTable)
	}
	config, playerCount, err := tableConfig(options)
	if err != nil {
		return nil, err
	}
	humanSeatOrder, humanSeats, err := normalizeHumanSeats(playerCount, options.HumanSeats)
	if err != nil {
		return nil, err
	}
	seriesID := options.SeriesID
	if seriesID == "" {
		seriesID = "table-series"
	}
	baseMatchID := options.BaseMatchID
	if baseMatchID == "" {
		baseMatchID = "table-match"
	}
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: seriesID,
		Seats:    canonicalTableSeats(playerCount),
		Config:   config,
	})
	if err != nil {
		return nil, err
	}
	controllers := maps.Clone(options.Controllers)
	if err := validateTableControllers(series.Seats(), humanSeats, controllers); err != nil {
		return nil, err
	}
	maxControllerTurns := options.MaxControllerTurns
	if maxControllerTurns <= 0 {
		maxControllerTurns = defaultMaxTableControllerTurns
	}
	t := &table{
		series:             series,
		controllers:        controllers,
		humanSeats:         humanSeats,
		humanSeatOrder:     humanSeatOrder,
		occupiedSeats:      make(map[domain.Seat]bool, len(humanSeatOrder)),
		baseMatchID:        baseMatchID,
		deal:               options.Deal,
		maxControllerTurns: maxControllerTurns,
	}
	if err := t.startNextMatch(ctx); err != nil {
		return nil, err
	}
	if err := t.advanceControllers(ctx); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *table) joinSeat(seat domain.Seat) (client.State, error) {
	if t == nil {
		return client.State{}, ErrTableNotFound
	}
	if !t.humanSeats[seat] {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	if t.occupiedSeats[seat] {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatOccupied, seat)
	}
	t.occupiedSeats[seat] = true
	return t.stateForSeat(seat), nil
}

func (t *table) releaseSeat(seat domain.Seat) error {
	if t == nil {
		return ErrTableNotFound
	}
	if !t.humanSeats[seat] {
		return fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	delete(t.occupiedSeats, seat)
	return nil
}

func (t *table) state(seat domain.Seat) (client.State, error) {
	if t == nil {
		return client.State{}, ErrTableNotFound
	}
	if !t.hasSeat(seat) {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	return t.stateForSeat(seat), nil
}

func (t *table) advance(ctx context.Context, seat domain.Seat) (client.State, error) {
	if err := ctx.Err(); err != nil {
		return client.State{}, err
	}
	if t == nil {
		return client.State{}, ErrTableNotFound
	}
	if !t.hasSeat(seat) {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	if err := t.advanceControllers(ctx); err != nil {
		return t.stateForSeat(seat), err
	}
	return t.stateForSeat(seat), nil
}

func (t *table) submitAction(
	ctx context.Context,
	seat domain.Seat,
	version uint64,
	actionID string,
) (client.State, error) {
	if err := ctx.Err(); err != nil {
		return client.State{}, err
	}
	if t == nil {
		return client.State{}, ErrTableNotFound
	}
	if !t.occupiedSeats[seat] {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	decision := t.session.DecisionContext(seat)
	state := client.StateFromDecision(t.matchID, t.version, &decision)
	if state.Version != version {
		return state, fmt.Errorf("%w: got %d want %d", ErrStaleState, version, state.Version)
	}
	action, err := actionByID(actionID, decision.LegalActions)
	if err != nil {
		return state, err
	}
	if err := t.session.ApplyAction(ctx, action); err != nil {
		return state, err
	}
	t.version++
	if err := t.advanceControllers(ctx); err != nil {
		return t.stateForSeat(seat), err
	}
	return t.stateForSeat(seat), nil
}

func (t *table) concede(ctx context.Context, seat domain.Seat) (client.State, error) {
	if err := ctx.Err(); err != nil {
		return client.State{}, err
	}
	if t == nil {
		return client.State{}, ErrTableNotFound
	}
	if !t.occupiedSeats[seat] {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	if err := t.session.Concede(ctx, seat); err != nil {
		return t.stateForSeat(seat), err
	}
	t.version++
	return t.stateForSeat(seat), nil
}

func (t *table) nextMatch(ctx context.Context, seat domain.Seat) (client.State, error) {
	if err := ctx.Err(); err != nil {
		return client.State{}, err
	}
	if t == nil {
		return client.State{}, ErrTableNotFound
	}
	if !t.occupiedSeats[seat] {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	if t.stateForSeat(seat).Phase != "complete" {
		return t.stateForSeat(seat), client.ErrMatchInProgress
	}
	if t.completedMatchID != t.matchID {
		if err := t.series.CompleteMatch(t.session); err != nil {
			return t.stateForSeat(seat), err
		}
		t.completedMatchID = t.matchID
	}
	if err := t.startNextMatch(ctx); err != nil {
		return t.stateForSeat(seat), err
	}
	t.completedMatchID = ""
	t.version++
	if err := t.advanceControllers(ctx); err != nil {
		return t.stateForSeat(seat), err
	}
	return t.stateForSeat(seat), nil
}

func (t *table) firstHumanSeat() domain.Seat {
	if t == nil || len(t.humanSeatOrder) == 0 {
		return domain.NoSeat
	}
	return t.humanSeatOrder[0]
}

func (t *table) stateForSeat(seat domain.Seat) client.State {
	if t == nil || t.session == nil {
		return client.State{}
	}
	decision := t.session.DecisionContext(seat)
	return client.StateFromDecision(t.matchID, t.version, &decision)
}

func (t *table) advanceControllers(ctx context.Context) error {
	for range t.maxControllerTurns {
		if err := ctx.Err(); err != nil {
			return err
		}
		if t.stateForSeat(t.firstHumanSeat()).Phase == "complete" {
			return nil
		}
		seat := t.activeControllerSeat()
		if seat == domain.NoSeat {
			return nil
		}
		controller := t.controllers[seat]
		if controller == nil {
			return fmt.Errorf("%w: seat %d", app.ErrMissingPlayerController, seat)
		}
		t.turnNumber++
		turn := t.turnContext(seat)
		controllerTurn := turn.Clone()
		decision, err := controller.Decide(ctx, &controllerTurn)
		if err != nil {
			return err
		}
		if err := t.session.ApplyPlayerDecision(ctx, seat, &turn, decision); err != nil {
			return err
		}
		t.version++
	}
	return fmt.Errorf("%w: after %d controller turns", app.ErrActionLimitExceeded, t.maxControllerTurns)
}

func (t *table) activeControllerSeat() domain.Seat {
	if t == nil || t.session == nil {
		return domain.NoSeat
	}
	view := t.session.ViewForSeat(t.firstHumanSeat())
	switch view.Phase {
	case domain.MatchPhaseAttack:
		return t.controllerSeat(view.Attacker)
	case domain.MatchPhaseDefense:
		return t.controllerSeat(view.Defender)
	case domain.MatchPhaseThrowIn, domain.MatchPhaseTaking:
		for _, seat := range throwInPollingOrder(view.Attacker, len(view.HandSizes)) {
			if seat == view.Defender {
				continue
			}
			if len(t.session.DecisionContext(seat).LegalActions) == 0 {
				continue
			}
			return t.controllerSeat(seat)
		}
		return domain.NoSeat
	default:
		return domain.NoSeat
	}
}

func (t *table) controllerSeat(seat domain.Seat) domain.Seat {
	if t.humanSeats[seat] {
		return domain.NoSeat
	}
	return seat
}

func (t *table) turnContext(seat domain.Seat) app.TurnContext {
	return app.TurnContext{
		SeriesID:        t.series.ID(),
		MatchID:         t.matchID,
		MatchNumber:     t.matchNumber,
		TurnNumber:      t.turnNumber,
		CanConcede:      true,
		DecisionContext: t.session.DecisionContext(seat),
	}
}

func (t *table) startNextMatch(ctx context.Context) error {
	matchNumber := t.matchNumber + 1
	t.turnNumber = 0
	matchID := tableMatchID(t.baseMatchID, matchNumber)
	session, _, err := t.series.StartMatch(ctx, &app.SeriesMatchOptions{
		MatchID: matchID,
		Deal:    t.deal,
	})
	if err != nil {
		return err
	}
	t.session = session
	t.matchID = matchID
	t.matchNumber = matchNumber
	return nil
}

func (t *table) hasSeat(seat domain.Seat) bool {
	return seat >= 0 && int(seat) < len(t.series.Seats())
}

func tableConfig(options *TableOptions) (app.MatchConfig, int, error) {
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
				ErrInvalidTable, config.Seats.PlayerCount, playerCount)
	}
	return config, playerCount, nil
}

func normalizeHumanSeats(
	playerCount int,
	seats []domain.Seat,
) ([]domain.Seat, map[domain.Seat]bool, error) {
	if len(seats) == 0 {
		seats = []domain.Seat{0}
	}
	humanSeatOrder := make([]domain.Seat, 0, len(seats))
	humanSeats := make(map[domain.Seat]bool, len(seats))
	for _, seat := range seats {
		if seat < 0 || int(seat) >= playerCount {
			return nil, nil, fmt.Errorf("%w: human seat %d outside 0..%d", ErrInvalidTable, seat, playerCount-1)
		}
		if humanSeats[seat] {
			return nil, nil, fmt.Errorf("%w: duplicate human seat %d", ErrInvalidTable, seat)
		}
		humanSeats[seat] = true
		humanSeatOrder = append(humanSeatOrder, seat)
	}
	return humanSeatOrder, humanSeats, nil
}

func validateTableControllers(
	seats []domain.Seat,
	humanSeats map[domain.Seat]bool,
	controllers map[domain.Seat]app.PlayerController,
) error {
	for _, seat := range seats {
		if humanSeats[seat] {
			continue
		}
		if controllers[seat] == nil {
			return fmt.Errorf("%w: seat %d", app.ErrMissingPlayerController, seat)
		}
	}
	return nil
}

func canonicalTableSeats(count int) []domain.Seat {
	seats := make([]domain.Seat, count)
	for seat := range seats {
		seats[seat] = domain.Seat(seat)
	}
	return seats
}

func actionByID(id string, actions []domain.Action) (domain.Action, error) {
	index, err := strconv.Atoi(id)
	if err != nil || index < 1 || index > len(actions) {
		return domain.Action{}, fmt.Errorf("%w: %q", client.ErrUnknownActionID, id)
	}
	return actions[index-1], nil
}

func tableMatchID(base app.MatchID, matchNumber int) app.MatchID {
	if matchNumber == 1 {
		return base
	}
	return app.MatchID(fmt.Sprintf("%s-%d", base, matchNumber))
}

func throwInPollingOrder(attacker domain.Seat, playerCount int) []domain.Seat {
	if playerCount <= 0 {
		return nil
	}
	order := make([]domain.Seat, 0, playerCount)
	start := ((int(attacker) % playerCount) + playerCount) % playerCount
	for offset := range playerCount {
		order = append(order, domain.Seat((start+offset)%playerCount))
	}
	return order
}
