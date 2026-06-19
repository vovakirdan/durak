package evaluation

import "github.com/vovakirdan/durak/internal/app"

// Evaluator keeps the historical evaluator entry point while delegating to the
// current risk model.
type Evaluator struct {
	Model RiskModel
}

// DefaultEvaluator creates the current seat-view risk evaluator.
func DefaultEvaluator() Evaluator {
	return Evaluator{Model: DefaultRiskModel()}
}

// Evaluate scores a position with the default risk model.
func Evaluate(decision *app.DecisionContext, hidden HiddenCards) PositionEvaluation {
	return DefaultEvaluator().Evaluate(decision, hidden)
}

// Evaluate scores a position with the configured risk model.
func (e Evaluator) Evaluate(decision *app.DecisionContext, hidden HiddenCards) PositionEvaluation {
	model := e.Model
	if model.Weights == (RiskWeights{}) {
		model = DefaultRiskModel()
	}
	return model.Evaluate(decision, hidden)
}

func hiddenConfidence(hidden HiddenCards) int {
	confidence := 100 - len(hidden.UnknownPool)*2
	if len(hidden.UnknownPool) > 0 && confidence < 20 {
		confidence = 20
	}
	return ClampConfidence(confidence)
}

func ensureHiddenCards(decision *app.DecisionContext, hidden HiddenCards) HiddenCards {
	if hidden.Known == nil && hidden.UnknownPool == nil {
		return BuildHiddenCards(decision, nil)
	}
	return hidden
}
