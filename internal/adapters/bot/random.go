package bot

import (
	"context"
	"fmt"
	rand "math/rand/v2"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// RandomLegalController chooses uniformly from legal actions for smoke testing.
type RandomLegalController struct {
	choose domain.IntnFunc
}

// NewRandomLegalController creates a random legal-action controller.
func NewRandomLegalController(choose domain.IntnFunc) RandomLegalController {
	if choose == nil {
		choose = rand.IntN
	}
	return RandomLegalController{choose: choose}
}

// Decide returns one legal action without evaluating the position.
func (c RandomLegalController) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	if err := ctx.Err(); err != nil {
		return app.PlayerDecision{}, err
	}
	if turn == nil {
		return app.PlayerDecision{}, app.ErrNilTurn
	}
	if len(turn.LegalActions) == 0 {
		return app.PlayerDecision{}, ErrNoLegalAction
	}
	index := c.choose(len(turn.LegalActions))
	if index < 0 || index >= len(turn.LegalActions) {
		return app.PlayerDecision{}, fmt.Errorf("%w: random chooser returned %d", app.ErrInvalidPlayerDecision, index)
	}
	return app.ActionDecision(turn.LegalActions[index]), nil
}
