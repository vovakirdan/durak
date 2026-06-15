package app

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/vovakirdan/durak/internal/domain"
)

const defaultMaxActionsPerMatch = 500

var (
	// ErrInvalidRunner means a headless runner was configured incorrectly.
	ErrInvalidRunner = errors.New("invalid runner")
	// ErrMissingPlayerController means no controller is configured for a seat.
	ErrMissingPlayerController = errors.New("missing player controller")
	// ErrActionLimitExceeded means a match did not finish before its guard limit.
	ErrActionLimitExceeded = errors.New("action limit exceeded")
)

// SeriesRunnerOptions configures headless series execution.
type SeriesRunnerOptions struct {
	Series             *Series
	Controllers        map[domain.Seat]PlayerController
	Deal               domain.DealOptions
	EventStore         EventStore
	InternalEventStore InternalEventStore
	BaseMatchID        MatchID
	MaxActionsPerMatch int
}

// SeriesRunner executes matches without CLI/TUI ownership.
type SeriesRunner struct {
	series             *Series
	controllers        map[domain.Seat]PlayerController
	deal               domain.DealOptions
	eventStore         EventStore
	internalEventStore InternalEventStore
	baseMatchID        MatchID
	maxActionsPerMatch int
	nextMatchNumber    int
}

// SeriesRunResult summarizes one headless run call.
type SeriesRunResult struct {
	Matches []SeriesMatchResult
	Turns   []TurnRecord
}

// TurnRecord is a compact trace of accepted runner decisions.
type TurnRecord struct {
	SeriesID    SeriesID
	MatchID     MatchID
	MatchNumber int
	TurnNumber  int
	Seat        domain.Seat
	Decision    PlayerDecision
}

// NewSeriesRunner creates a reusable headless runner for one series.
func NewSeriesRunner(options *SeriesRunnerOptions) (*SeriesRunner, error) {
	if options == nil {
		return nil, fmt.Errorf("%w: options are nil", ErrInvalidRunner)
	}
	if options.Series == nil {
		return nil, fmt.Errorf("%w: series is nil", ErrInvalidRunner)
	}
	controllers := maps.Clone(options.Controllers)
	if err := validateRunnerControllers(options.Series.Seats(), controllers); err != nil {
		return nil, err
	}
	maxActions := options.MaxActionsPerMatch
	if maxActions <= 0 {
		maxActions = defaultMaxActionsPerMatch
	}
	return &SeriesRunner{
		series:             options.Series,
		controllers:        controllers,
		deal:               options.Deal,
		eventStore:         options.EventStore,
		internalEventStore: options.InternalEventStore,
		baseMatchID:        options.BaseMatchID,
		maxActionsPerMatch: maxActions,
	}, nil
}

// Run plays matchCount matches through configured controllers.
func (r *SeriesRunner) Run(ctx context.Context, matchCount int) (SeriesRunResult, error) {
	if r == nil {
		return SeriesRunResult{}, fmt.Errorf("%w: runner is nil", ErrInvalidRunner)
	}
	if matchCount <= 0 {
		return SeriesRunResult{}, fmt.Errorf("%w: match count must be positive", ErrInvalidRunner)
	}
	result := SeriesRunResult{
		Matches: make([]SeriesMatchResult, 0, matchCount),
	}
	for range matchCount {
		matchNumber := r.nextMatchNumber + 1
		matchID := runnerMatchID(r.baseMatchID, matchNumber)
		session, _, err := r.series.StartMatch(ctx, SeriesMatchOptions{
			MatchID:            matchID,
			Deal:               r.deal,
			EventStore:         r.eventStore,
			InternalEventStore: r.internalEventStore,
		})
		if err != nil {
			return result, err
		}
		r.nextMatchNumber = matchNumber

		turns, err := r.runMatch(ctx, session, matchID, matchNumber)
		result.Turns = append(result.Turns, turns...)
		if err != nil {
			return result, err
		}
		if err := r.series.CompleteMatch(session); err != nil {
			return result, err
		}
		results := r.series.Results()
		result.Matches = append(result.Matches, results[len(results)-1])
	}
	return result, nil
}

func validateRunnerControllers(seats []domain.Seat, controllers map[domain.Seat]PlayerController) error {
	for _, seat := range seats {
		if controllers[seat] == nil {
			return fmt.Errorf("%w: seat %d", ErrMissingPlayerController, seat)
		}
	}
	return nil
}

func (r *SeriesRunner) runMatch(
	ctx context.Context,
	session *Session,
	matchID MatchID,
	matchNumber int,
) ([]TurnRecord, error) {
	turns := make([]TurnRecord, 0)
	for turnNumber := 1; ; turnNumber++ {
		if err := ctx.Err(); err != nil {
			return turns, err
		}
		view := session.ViewForSeat(0)
		if view.Phase == domain.MatchPhaseComplete {
			return turns, nil
		}
		if turnNumber > r.maxActionsPerMatch {
			return turns, fmt.Errorf("%w: match %q after %d actions", ErrActionLimitExceeded, matchID, r.maxActionsPerMatch)
		}

		seat := activeRunnerSeat(session, &view)
		controller := r.controllers[seat]
		if controller == nil {
			return turns, fmt.Errorf("%w: seat %d", ErrMissingPlayerController, seat)
		}
		turn := r.turnContext(session, seat, matchID, matchNumber, turnNumber)
		controllerTurn := turn.Clone()
		decision, err := controller.Decide(ctx, &controllerTurn)
		if err != nil {
			return turns, err
		}
		if err := session.ApplyPlayerDecision(ctx, seat, &turn, decision); err != nil {
			return turns, err
		}
		turns = append(turns, TurnRecord{
			SeriesID:    r.series.ID(),
			MatchID:     matchID,
			MatchNumber: matchNumber,
			TurnNumber:  turnNumber,
			Seat:        seat,
			Decision:    decision,
		})
	}
}

func (r *SeriesRunner) turnContext(
	session *Session,
	seat domain.Seat,
	matchID MatchID,
	matchNumber int,
	turnNumber int,
) TurnContext {
	decision := session.DecisionContext(seat)
	return TurnContext{
		SeriesID:        r.series.ID(),
		MatchID:         matchID,
		MatchNumber:     matchNumber,
		TurnNumber:      turnNumber,
		CanConcede:      true,
		DecisionContext: decision,
	}
}

func activeRunnerSeat(session *Session, view *SeatView) domain.Seat {
	if view == nil {
		return domain.NoSeat
	}
	switch view.Phase {
	case domain.MatchPhaseAttack:
		return view.Attacker
	case domain.MatchPhaseThrowIn, domain.MatchPhaseTaking:
		return activeThrowInRunnerSeat(session, view)
	case domain.MatchPhaseDefense:
		return view.Defender
	case domain.MatchPhaseComplete:
		return domain.NoSeat
	default:
		return domain.NoSeat
	}
}

func activeThrowInRunnerSeat(session *Session, view *SeatView) domain.Seat {
	if session == nil || view == nil {
		return domain.NoSeat
	}
	for _, seat := range throwInPollingOrder(view.Attacker, len(view.HandSizes)) {
		if seat == view.Defender {
			continue
		}
		if len(session.DecisionContext(seat).LegalActions) > 0 {
			return seat
		}
	}
	return domain.NoSeat
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

func runnerMatchID(base MatchID, matchNumber int) MatchID {
	if base == "" {
		return MatchID(fmt.Sprintf("runner-match-%d", matchNumber))
	}
	if matchNumber == 1 {
		return base
	}
	return MatchID(fmt.Sprintf("%s-%d", base, matchNumber))
}
