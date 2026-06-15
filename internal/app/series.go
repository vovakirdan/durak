package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

// SeriesID identifies a consecutive set of matches at one table.
type SeriesID string

// ErrInvalidSeries means a series or series transition is invalid.
var ErrInvalidSeries = errors.New("invalid series")

// SeriesOptions configures an in-memory match series.
type SeriesOptions struct {
	SeriesID SeriesID
	Seats    []domain.Seat
	Config   MatchConfig
}

// SeriesMatchOptions configures one match inside a series.
type SeriesMatchOptions struct {
	MatchID            MatchID
	Deal               domain.DealOptions
	EventStore         EventStore
	InternalEventStore InternalEventStore
}

// SeriesMatchResult records a completed match in a series.
type SeriesMatchResult struct {
	MatchID MatchID
	Winner  domain.Seat
	Loser   domain.Seat
	Draw    bool
}

// Series owns table-level state that links optional consecutive matches.
type Series struct {
	id               SeriesID
	seats            []domain.Seat
	config           MatchConfig
	configIdentity   MatchConfigIdentity
	profile          domain.RuleProfile
	results          []SeriesMatchResult
	previousLoser    domain.Seat
	hasPreviousLoser bool
}

// NewSeries creates an in-memory series with canonical seats 0..n-1.
func NewSeries(options *SeriesOptions) (*Series, error) {
	if options == nil {
		return nil, fmt.Errorf("%w: options are nil", ErrInvalidSeries)
	}
	if options.SeriesID == "" {
		return nil, fmt.Errorf("%w: series id is empty", ErrInvalidSeries)
	}
	config, seats, profile, err := normalizeSeriesConfig(options)
	if err != nil {
		return nil, err
	}
	if seatErr := validateSeriesSeats(seats, profile); seatErr != nil {
		return nil, seatErr
	}
	configIdentity, err := config.Identity()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidSeries, err)
	}
	return &Series{
		id:             options.SeriesID,
		seats:          slices.Clone(seats),
		config:         config,
		configIdentity: configIdentity,
		profile:        profile,
	}, nil
}

// ID returns the stable series identifier.
func (s *Series) ID() SeriesID {
	if s == nil {
		return ""
	}
	return s.id
}

// Seats returns the stable seat order for consecutive-match rules.
func (s *Series) Seats() []domain.Seat {
	if s == nil {
		return nil
	}
	return slices.Clone(s.seats)
}

// Config returns the immutable match configuration used by this series.
func (s *Series) Config() MatchConfig {
	if s == nil {
		return MatchConfig{}
	}
	return s.config
}

// ConfigIdentity returns the stable identity for matches started by this series.
func (s *Series) ConfigIdentity() MatchConfigIdentity {
	if s == nil {
		return MatchConfigIdentity{}
	}
	return s.configIdentity
}

// PreviousLoser returns the last completed match loser, if the last match had one.
func (s *Series) PreviousLoser() (domain.Seat, bool) {
	if s == nil || !s.hasPreviousLoser {
		return domain.NoSeat, false
	}
	return s.previousLoser, true
}

// Results returns completed match results in series order.
func (s *Series) Results() []SeriesMatchResult {
	if s == nil {
		return nil
	}
	return slices.Clone(s.results)
}

// StartMatch deals and starts one match in the series.
func (s *Series) StartMatch(ctx context.Context, options SeriesMatchOptions) (*Session, domain.InitialDeal, error) {
	if s == nil {
		return nil, domain.InitialDeal{}, fmt.Errorf("%w: series is nil", ErrInvalidSeries)
	}
	if options.MatchID == "" {
		return nil, domain.InitialDeal{}, ErrEmptyMatchID
	}
	if s.hasResult(options.MatchID) {
		return nil, domain.InitialDeal{}, fmt.Errorf("%w: match %q already completed", ErrInvalidSeries, options.MatchID)
	}
	deal, err := domain.DealInitial(len(s.seats), s.profile, options.Deal)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}
	if s.config.Series.Consecutive && s.hasPreviousLoser {
		deal.FirstAttacker = int(s.seatBefore(s.previousLoser))
		deal.RandomFirstAttacker = false
	}

	match, err := domain.NewMatch(&deal, s.profile)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}
	session, err := NewSessionWithOptions(ctx, match, SessionOptions{
		MatchID:            options.MatchID,
		ConfigIdentity:     &s.configIdentity,
		EventStore:         options.EventStore,
		InternalEventStore: options.InternalEventStore,
		InitialDeal:        &deal,
	})
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}
	return session, deal, nil
}

// CompleteMatch records a completed match and updates consecutive-match state.
func (s *Series) CompleteMatch(session *Session) error {
	if s == nil {
		return fmt.Errorf("%w: series is nil", ErrInvalidSeries)
	}
	if session == nil {
		return fmt.Errorf("%w: session is nil", ErrInvalidSeries)
	}
	if session.matchID == "" {
		return ErrEmptyMatchID
	}
	if s.hasResult(session.matchID) {
		return fmt.Errorf("%w: match %q already completed", ErrInvalidSeries, session.matchID)
	}
	view := session.ViewForSeat(s.seats[0])
	if view.Phase != domain.MatchPhaseComplete {
		return fmt.Errorf("%w: match %q is not complete", ErrInvalidSeries, session.matchID)
	}
	result := SeriesMatchResult{
		MatchID: session.matchID,
		Winner:  view.Winner,
		Loser:   view.Loser,
		Draw:    view.Winner == domain.NoSeat || view.Loser == domain.NoSeat,
	}
	s.results = append(s.results, result)
	if result.Draw {
		s.previousLoser = domain.NoSeat
		s.hasPreviousLoser = false
		return nil
	}
	if !s.hasSeat(result.Loser) {
		return fmt.Errorf("%w: loser %d is outside series seats", ErrInvalidSeries, result.Loser)
	}
	s.previousLoser = result.Loser
	s.hasPreviousLoser = true
	return nil
}

func normalizeSeriesConfig(options *SeriesOptions) (MatchConfig, []domain.Seat, domain.RuleProfile, error) {
	seats := slices.Clone(options.Seats)
	config := options.Config
	if config == (MatchConfig{}) {
		playerCount := len(seats)
		if playerCount == 0 {
			playerCount = 2
		}
		var err error
		config, err = NewMatchConfig(RulePresetDefault, playerCount)
		if err != nil {
			return MatchConfig{}, nil, domain.RuleProfile{}, fmt.Errorf("%w: %w", ErrInvalidSeries, err)
		}
	}
	if len(seats) == 0 {
		seats = canonicalSeriesSeats(config.Seats.PlayerCount)
	}
	if config.Seats.PlayerCount != len(seats) {
		return MatchConfig{}, nil, domain.RuleProfile{},
			fmt.Errorf("%w: config seats %d do not match seat order %d",
				ErrInvalidSeries, config.Seats.PlayerCount, len(seats))
	}
	profile, err := config.RuleProfile()
	if err != nil {
		return MatchConfig{}, nil, domain.RuleProfile{}, fmt.Errorf("%w: %w", ErrInvalidSeries, err)
	}
	return config, seats, profile, nil
}

func canonicalSeriesSeats(count int) []domain.Seat {
	seats := make([]domain.Seat, count)
	for seat := range seats {
		seats[seat] = domain.Seat(seat)
	}
	return seats
}

func validateSeriesSeats(seats []domain.Seat, profile domain.RuleProfile) error {
	if len(seats) < 2 || len(seats) > profile.MaxPlayers {
		return fmt.Errorf("%w: got %d seats, allowed 2..%d", ErrInvalidSeries, len(seats), profile.MaxPlayers)
	}
	seen := make(map[domain.Seat]bool, len(seats))
	for index, seat := range seats {
		if seat != domain.Seat(index) {
			return fmt.Errorf("%w: seats must be canonical 0..%d", ErrInvalidSeries, len(seats)-1)
		}
		if seen[seat] {
			return fmt.Errorf("%w: duplicate seat %d", ErrInvalidSeries, seat)
		}
		seen[seat] = true
	}
	return nil
}

func (s *Series) hasSeat(seat domain.Seat) bool {
	return slices.Contains(s.seats, seat)
}

func (s *Series) hasResult(matchID MatchID) bool {
	return slices.ContainsFunc(s.results, func(result SeriesMatchResult) bool {
		return result.MatchID == matchID
	})
}

func (s *Series) seatBefore(seat domain.Seat) domain.Seat {
	index := slices.Index(s.seats, seat)
	if index <= 0 {
		return s.seats[len(s.seats)-1]
	}
	return s.seats[index-1]
}
