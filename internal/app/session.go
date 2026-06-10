package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

var (
	// ErrNilMatch means a session was created without a match.
	ErrNilMatch = errors.New("nil match")
	// ErrNilStrategy means a bot turn was requested without a strategy.
	ErrNilStrategy = errors.New("nil strategy")
	// ErrIllegalAction means a strategy selected an action outside legal actions.
	ErrIllegalAction = errors.New("illegal action")
)

// Strategy chooses an action from a read-only decision context.
type Strategy interface {
	ChooseAction(context.Context, *DecisionContext) (domain.Action, error)
}

// Session orchestrates one active match for adapters and bots.
type Session struct {
	match *domain.Match
}

// NewSession wraps an existing domain match.
func NewSession(match *domain.Match) (*Session, error) {
	if match == nil {
		return nil, ErrNilMatch
	}
	return &Session{match: match}, nil
}

// NewDealtSession creates a match by dealing cards with the provided profile.
func NewDealtSession(playerCount int, profile domain.RuleProfile, opts domain.DealOptions) (*Session, domain.InitialDeal, error) {
	deal, err := domain.DealInitial(playerCount, profile, opts)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}

	match, err := domain.NewMatch(&deal, profile)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}

	session, err := NewSession(match)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}
	return session, deal, nil
}

// ApplyAction validates and applies an action through the domain match.
func (s *Session) ApplyAction(ctx context.Context, action domain.Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.match.ApplyAction(action)
}

// ApplyStrategy asks strategy for one legal action and applies it.
func (s *Session) ApplyStrategy(ctx context.Context, seat domain.Seat, strategy Strategy) (domain.Action, error) {
	if err := ctx.Err(); err != nil {
		return domain.Action{}, err
	}
	if strategy == nil {
		return domain.Action{}, ErrNilStrategy
	}

	decision := s.DecisionContext(seat)
	action, err := strategy.ChooseAction(ctx, &decision)
	if err != nil {
		return domain.Action{}, err
	}
	if !slices.Contains(decision.LegalActions, action) {
		return domain.Action{}, fmt.Errorf("%w: %v", ErrIllegalAction, action.Kind)
	}

	return action, s.ApplyAction(ctx, action)
}

// DecisionContext returns read-only information needed to choose an action.
func (s *Session) DecisionContext(seat domain.Seat) DecisionContext {
	view := s.ViewForSeat(seat)
	return DecisionContext{
		SeatView:     view,
		Hand:         s.match.Hand(seat),
		LegalActions: slices.Clone(s.match.LegalActions(seat)),
	}
}

// ViewForSeat returns a public seat-specific view of the match.
func (s *Session) ViewForSeat(seat domain.Seat) SeatView {
	return SeatView{
		Seat:               seat,
		Phase:              s.match.Phase(),
		Attacker:           s.match.Attacker(),
		Defender:           s.match.Defender(),
		TrumpSuit:          s.match.TrumpSuit(),
		TrumpIndicator:     s.match.TrumpIndicator(),
		Table:              s.match.Table(),
		HandSizes:          s.handSizes(),
		StockCount:         s.match.StockCount(),
		DiscardCount:       s.match.DiscardCount(),
		SuccessfulDefenses: s.match.SuccessfulDefenses(),
		Winner:             s.match.Winner(),
		Loser:              s.match.Loser(),
	}
}

func (s *Session) handSizes() []int {
	sizes := make([]int, s.match.PlayerCount())
	for seat := range sizes {
		sizes[seat] = s.match.HandSize(domain.Seat(seat))
	}
	return sizes
}
