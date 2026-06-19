package bot

import (
	"context"
	"errors"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// ErrNoLegalAction means the strategy received no action candidates.
var ErrNoLegalAction = errors.New("no legal action")

// SimpleStrategy is a deterministic baseline bot for the first CLI loop.
type SimpleStrategy struct{}

// NewSimpleStrategy creates the baseline deterministic bot strategy.
func NewSimpleStrategy() SimpleStrategy {
	return SimpleStrategy{}
}

// ChooseAction returns the simplest useful action from the decision context.
func (SimpleStrategy) ChooseAction(ctx context.Context, decision *app.DecisionContext) (domain.Action, error) {
	if err := ctx.Err(); err != nil {
		return domain.Action{}, err
	}
	if decision == nil {
		return domain.Action{}, ErrNoLegalAction
	}
	if len(decision.LegalActions) == 0 {
		return domain.Action{}, ErrNoLegalAction
	}

	best := decision.LegalActions[0]
	for _, action := range decision.LegalActions[1:] {
		if preferAction(action, best, decision.TrumpSuit) {
			best = action
		}
	}
	return best, nil
}

func preferAction(candidate, current domain.Action, trumpSuit domain.Suit) bool {
	candidatePriority := actionPriority(candidate.Kind)
	currentPriority := actionPriority(current.Kind)
	if candidatePriority != currentPriority {
		return candidatePriority < currentPriority
	}
	return actionLess(candidate, current, trumpSuit)
}

func actionPriority(kind domain.ActionKind) int {
	switch kind {
	case domain.ActionKindDefend:
		return 0
	case domain.ActionKindTransfer:
		return 1
	case domain.ActionKindAttack:
		return 2
	case domain.ActionKindThrowIn:
		return 3
	case domain.ActionKindPassThrowIn:
		return 4
	case domain.ActionKindFinishDefense:
		return 5
	case domain.ActionKindFinishTake:
		return 6
	case domain.ActionKindTake:
		return 7
	default:
		return 8
	}
}

func actionLess(left, right domain.Action, trumpSuit domain.Suit) bool {
	if left.AttackIndex != right.AttackIndex {
		return left.AttackIndex < right.AttackIndex
	}
	if left.Kind == domain.ActionKindAttack && right.Kind == domain.ActionKindAttack {
		leftCount := len(left.AttackCards())
		rightCount := len(right.AttackCards())
		if leftCount != rightCount {
			return leftCount < rightCount
		}
	}
	return cardLess(left.Card, right.Card, trumpSuit)
}

func cardLess(left, right domain.Card, trumpSuit domain.Suit) bool {
	leftTrump := left.Suit == trumpSuit
	rightTrump := right.Suit == trumpSuit
	if leftTrump != rightTrump {
		return !leftTrump
	}
	if left.Rank != right.Rank {
		return left.Rank < right.Rank
	}
	return left.Suit < right.Suit
}
