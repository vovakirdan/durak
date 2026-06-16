package bot

import (
	"context"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
)

// HeuristicController chooses the highest ranked action from the static evaluator.
type HeuristicController struct{}

// NewHeuristicController creates an explainable offline bot controller.
func NewHeuristicController() HeuristicController {
	return HeuristicController{}
}

// Decide ranks legal actions from the visible seat view and picks the best one.
func (HeuristicController) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	if err := ctx.Err(); err != nil {
		return app.PlayerDecision{}, err
	}
	if turn == nil {
		return app.PlayerDecision{}, app.ErrNilTurn
	}
	if len(turn.LegalActions) == 0 {
		return app.PlayerDecision{}, ErrNoLegalAction
	}
	decision := &turn.DecisionContext
	hidden := evaluation.BuildHiddenCards(decision, nil)
	ranked := evaluation.RankActions(decision, hidden)
	if len(ranked) == 0 {
		return app.PlayerDecision{}, ErrNoLegalAction
	}
	return app.ActionDecision(ranked[0].Action), nil
}
