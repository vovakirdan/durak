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
	// ErrEmptyMatchID means an event store was configured without a match stream id.
	ErrEmptyMatchID = errors.New("empty match id")
	// ErrMissingInitialDeal means internal event storage lacks full setup state.
	ErrMissingInitialDeal = errors.New("missing initial deal")
)

// Strategy chooses an action from a read-only decision context.
type Strategy interface {
	ChooseAction(context.Context, *DecisionContext) (domain.Action, error)
}

// Session orchestrates one active match for adapters and bots.
type Session struct {
	match          *domain.Match
	matchID        MatchID
	seriesID       SeriesID
	configIdentity MatchConfigIdentity
	configSnapshot *MatchConfigSnapshot
	eventStore     EventStore
	internalStore  InternalEventStore
	matchRecorder  MatchRecorder
	publicEvents   []Event
	publicHistory  PublicCardHistory
	initialDeal    *domain.InitialDeal
	nextSequence   uint64
}

// SessionOptions configures optional session ports. EventStore and
// InternalEventStore are independent ports; durable adapters should not rely on
// them as an atomic cross-store transaction.
type SessionOptions struct {
	MatchID            MatchID
	SeriesID           SeriesID
	ConfigIdentity     *MatchConfigIdentity
	ConfigSnapshot     *MatchConfigSnapshot
	EventStore         EventStore
	InternalEventStore InternalEventStore
	MatchRecorder      MatchRecorder
	InitialDeal        *domain.InitialDeal
}

// NewSession wraps an existing domain match.
func NewSession(match *domain.Match) (*Session, error) {
	return NewSessionWithOptions(context.Background(), match, nil)
}

// NewSessionWithOptions wraps an existing domain match and emits initial events.
func NewSessionWithOptions(ctx context.Context, match *domain.Match, options *SessionOptions) (*Session, error) {
	if match == nil {
		return nil, ErrNilMatch
	}
	if options == nil {
		options = &SessionOptions{}
	}
	if options.EventStore != nil && options.MatchID == "" {
		return nil, ErrEmptyMatchID
	}
	if options.InternalEventStore != nil && options.MatchID == "" {
		return nil, ErrEmptyMatchID
	}
	if options.InternalEventStore != nil && options.InitialDeal == nil {
		return nil, ErrMissingInitialDeal
	}
	if options.MatchRecorder != nil && options.MatchID == "" {
		return nil, ErrEmptyMatchID
	}
	if options.MatchRecorder != nil && options.InitialDeal == nil {
		return nil, ErrMissingInitialDeal
	}
	if options.MatchRecorder != nil && options.ConfigSnapshot == nil {
		return nil, ErrMissingConfigSnapshot
	}
	configIdentity := MatchConfigIdentity{}
	if options.ConfigSnapshot != nil {
		configIdentity = options.ConfigSnapshot.Identity
	} else if options.ConfigIdentity != nil {
		configIdentity = *options.ConfigIdentity
	}
	session := &Session{
		match:          match,
		matchID:        options.MatchID,
		seriesID:       options.SeriesID,
		configIdentity: configIdentity,
		configSnapshot: cloneConfigSnapshot(options.ConfigSnapshot),
		eventStore:     options.EventStore,
		internalStore:  options.InternalEventStore,
		matchRecorder:  options.MatchRecorder,
		initialDeal:    cloneInitialDeal(options.InitialDeal),
	}
	if err := session.emitPendingEvents(ctx); err != nil {
		return nil, err
	}
	return session, nil
}

// NewDealtSession creates a match by dealing cards with the provided profile.
func NewDealtSession(playerCount int, profile domain.RuleProfile, opts domain.DealOptions) (*Session, domain.InitialDeal, error) {
	return NewDealtSessionWithOptions(context.Background(), playerCount, profile, opts, nil)
}

// NewDealtSessionWithOptions creates a match and emits initial events.
func NewDealtSessionWithOptions(
	ctx context.Context,
	playerCount int,
	profile domain.RuleProfile,
	opts domain.DealOptions,
	sessionOptions *SessionOptions,
) (*Session, domain.InitialDeal, error) {
	deal, err := domain.DealInitial(playerCount, profile, opts)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}

	match, err := domain.NewMatch(&deal, profile)
	if err != nil {
		return nil, domain.InitialDeal{}, err
	}

	options := SessionOptions{}
	if sessionOptions != nil {
		options = *sessionOptions
	}
	options.InitialDeal = &deal
	session, err := NewSessionWithOptions(ctx, match, &options)
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
	if err := s.match.ApplyAction(action); err != nil {
		return err
	}
	return s.emitPendingEvents(ctx)
}

// Concede completes the match by concession for the given seat.
func (s *Session) Concede(ctx context.Context, seat domain.Seat) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.match.Concede(seat); err != nil {
		return err
	}
	return s.emitPendingEvents(ctx)
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
	decision := DecisionContext{
		SeatView:     view,
		Hand:         s.match.Hand(seat),
		LegalActions: slices.Clone(s.match.LegalActions(seat)),
	}
	decision.PublicMemory = s.publicHistory.Snapshot(seat, &decision)
	return decision
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

func (s *Session) emitPendingEvents(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	events := s.match.Events()
	if len(events) == 0 {
		return nil
	}
	if s.eventStore == nil {
		if s.internalStore == nil && s.matchRecorder == nil {
			s.applyPublicHistory(events)
			s.match.DrainEvents()
			return nil
		}
	}
	storedEvents := make([]Event, len(events))
	for i, event := range events {
		storedEvents[i] = Event{
			MatchID:        s.matchID,
			Sequence:       s.nextSequence + uint64(i) + 1,
			ConfigIdentity: s.eventConfigIdentity(event),
			Domain:         event,
		}
	}
	var internalEvents []InternalEvent
	if s.internalStore != nil || s.matchRecorder != nil {
		var err error
		internalEvents, err = s.internalEvents(events)
		if err != nil {
			return err
		}
	}
	if s.matchRecorder != nil {
		batch, err := s.matchRecordBatch(storedEvents, internalEvents)
		if err != nil {
			return err
		}
		if err := s.matchRecorder.RecordMatchBatch(ctx, &batch); err != nil {
			return err
		}
	}
	if s.internalStore != nil {
		if err := s.internalStore.AppendInternalEvents(ctx, internalEvents); err != nil {
			return err
		}
	}
	if s.eventStore != nil {
		if err := s.eventStore.AppendEvents(ctx, storedEvents); err != nil {
			return err
		}
	}
	s.applyPublicHistory(events)
	s.nextSequence += uint64(len(events))
	if s.matchRecorder != nil {
		s.publicEvents = append(s.publicEvents, cloneEvents(storedEvents)...)
	}
	s.match.DrainEvents()
	return nil
}

func (s *Session) applyPublicHistory(events []domain.Event) {
	for _, event := range events {
		s.publicHistory.Apply(event)
	}
}

func (s *Session) matchRecordBatch(publicEvents []Event, internalEvents []InternalEvent) (MatchRecordBatch, error) {
	summary, err := s.summaryForBatch(publicEvents)
	if err != nil {
		return MatchRecordBatch{}, err
	}
	return MatchRecordBatch{
		MatchID:        s.matchID,
		SeriesID:       s.seriesID,
		ConfigSnapshot: s.configSnapshotForBatch(publicEvents),
		PublicEvents:   cloneEvents(publicEvents),
		InternalEvents: cloneInternalEvents(internalEvents),
		Summary:        summary,
	}, nil
}

func (s *Session) configSnapshotForBatch(events []Event) *MatchConfigSnapshot {
	for i := range events {
		if events[i].Domain.Kind == domain.EventKindMatchStarted {
			return cloneConfigSnapshot(s.configSnapshot)
		}
	}
	return nil
}

func (s *Session) summaryForBatch(events []Event) (*MatchSummary, error) {
	if len(events) == 0 || events[len(events)-1].Domain.Kind != domain.EventKindMatchEnded {
		return nil, nil
	}
	allEvents := append(cloneEvents(s.publicEvents), cloneEvents(events)...)
	summary, err := BuildMatchSummary(allEvents)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func (s *Session) internalEvents(events []domain.Event) ([]InternalEvent, error) {
	storedEvents := make([]InternalEvent, len(events))
	for i, event := range events {
		internalEvent := InternalEvent{
			MatchID:        s.matchID,
			Sequence:       s.nextSequence + uint64(i) + 1,
			ConfigIdentity: s.eventConfigIdentity(event),
			Domain:         event,
		}
		if event.Kind == domain.EventKindDeal {
			if s.initialDeal == nil {
				return nil, ErrMissingInitialDeal
			}
			defender := domain.NoSeat
			if event.Deal != nil {
				defender = event.Deal.Defender
			}
			deal := NewInternalDealEvent(s.initialDeal, defender)
			internalEvent.Deal = &deal
		}
		storedEvents[i] = internalEvent
	}
	return storedEvents, nil
}

func (s *Session) eventConfigIdentity(event domain.Event) MatchConfigIdentity {
	if event.Kind == domain.EventKindMatchStarted {
		return s.configIdentity
	}
	return MatchConfigIdentity{}
}

func cloneInitialDeal(deal *domain.InitialDeal) *domain.InitialDeal {
	if deal == nil {
		return nil
	}
	return &domain.InitialDeal{
		Hands:               cloneHands(deal.Hands),
		Stock:               slices.Clone(deal.Stock),
		TrumpIndicator:      deal.TrumpIndicator,
		TrumpSuit:           deal.TrumpSuit,
		FirstAttacker:       deal.FirstAttacker,
		Redeals:             deal.Redeals,
		TrumpReselections:   deal.TrumpReselections,
		RandomFirstAttacker: deal.RandomFirstAttacker,
	}
}
