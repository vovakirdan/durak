package app

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

var (
	// ErrInvalidPlayerDecision means a controller returned an unusable decision.
	ErrInvalidPlayerDecision = errors.New("invalid player decision")
	// ErrNilTurn means a controller received no turn context.
	ErrNilTurn = errors.New("nil turn")
)

// PlayerController chooses one player decision for an active seat.
type PlayerController interface {
	Decide(context.Context, *TurnContext) (PlayerDecision, error)
}

// PlayerDecisionKind identifies a controller decision type.
type PlayerDecisionKind uint8

const (
	// PlayerDecisionUnknown is the zero value for an unset decision.
	PlayerDecisionUnknown PlayerDecisionKind = iota
	// PlayerDecisionAction applies a domain action through normal validation.
	PlayerDecisionAction
	// PlayerDecisionConcede gives up the current match.
	PlayerDecisionConcede
)

// PlayerDecision is a controller command selected from a turn context.
type PlayerDecision struct {
	Kind   PlayerDecisionKind
	Action domain.Action
}

// ActionDecision wraps a legal domain action as a player decision.
func ActionDecision(action domain.Action) PlayerDecision {
	return PlayerDecision{Kind: PlayerDecisionAction, Action: action}
}

// ConcedeDecision gives up the current match for the active seat.
func ConcedeDecision() PlayerDecision {
	return PlayerDecision{Kind: PlayerDecisionConcede}
}

// TurnContext is the read-only state a controller sees for one active turn.
type TurnContext struct {
	SeriesID    SeriesID
	MatchID     MatchID
	MatchNumber int
	TurnNumber  int
	CanConcede  bool
	DecisionContext
}

// Clone returns a deep copy suitable for giving to untrusted controllers.
func (t *TurnContext) Clone() TurnContext {
	return cloneTurnContext(t)
}

// ApplyPlayerDecision validates and applies a controller decision.
func (s *Session) ApplyPlayerDecision(
	ctx context.Context,
	seat domain.Seat,
	turn *TurnContext,
	decision PlayerDecision,
) error {
	if turn == nil {
		return ErrNilTurn
	}
	switch decision.Kind {
	case PlayerDecisionAction:
		if !slices.Contains(turn.LegalActions, decision.Action) {
			return fmt.Errorf("%w: %v", ErrIllegalAction, decision.Action.Kind)
		}
		return s.ApplyAction(ctx, decision.Action)
	case PlayerDecisionConcede:
		if !turn.CanConcede {
			return fmt.Errorf("%w: concede is not available", ErrInvalidPlayerDecision)
		}
		return s.Concede(ctx, seat)
	default:
		return fmt.Errorf("%w: kind %d", ErrInvalidPlayerDecision, decision.Kind)
	}
}

// StrategyController adapts the existing strategy interface into a player
// controller. Strategy selection stays separate from game orchestration.
type StrategyController struct {
	Strategy Strategy
}

// Decide asks the wrapped strategy for a domain action.
func (c StrategyController) Decide(ctx context.Context, turn *TurnContext) (PlayerDecision, error) {
	if turn == nil {
		return PlayerDecision{}, ErrNilTurn
	}
	if c.Strategy == nil {
		return PlayerDecision{}, ErrNilStrategy
	}
	decision := cloneDecisionContext(&turn.DecisionContext)
	action, err := c.Strategy.ChooseAction(ctx, &decision)
	if err != nil {
		return PlayerDecision{}, err
	}
	return ActionDecision(action), nil
}

func cloneTurnContext(turn *TurnContext) TurnContext {
	if turn == nil {
		return TurnContext{}
	}
	cloned := *turn
	cloned.DecisionContext = cloneDecisionContext(&turn.DecisionContext)
	return cloned
}

func cloneDecisionContext(decision *DecisionContext) DecisionContext {
	if decision == nil {
		return DecisionContext{}
	}
	return DecisionContext{
		SeatView:     cloneSeatView(&decision.SeatView),
		Hand:         slices.Clone(decision.Hand),
		LegalActions: slices.Clone(decision.LegalActions),
		PublicMemory: decision.PublicMemory.Clone(),
	}
}

func cloneSeatView(view *SeatView) SeatView {
	if view == nil {
		return SeatView{}
	}
	return SeatView{
		Seat:               view.Seat,
		Phase:              view.Phase,
		Attacker:           view.Attacker,
		Defender:           view.Defender,
		TrumpSuit:          view.TrumpSuit,
		TrumpIndicator:     view.TrumpIndicator,
		Table:              slices.Clone(view.Table),
		HandSizes:          slices.Clone(view.HandSizes),
		StockCount:         view.StockCount,
		DiscardCount:       view.DiscardCount,
		SuccessfulDefenses: view.SuccessfulDefenses,
		Winner:             view.Winner,
		Loser:              view.Loser,
	}
}
