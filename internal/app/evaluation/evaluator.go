package evaluation

import (
	"fmt"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// Weights are explicit hand-tuned constants for the first evaluator.
type Weights struct {
	MaterialPerCard       Score
	TrumpPerCard          Score
	TrumpRankScale        Score
	DefenseCovered        Score
	DefenseMissed         Score
	AttackAction          Score
	RoleAttacker          Score
	RoleDefender          Score
	TakingRisk            Score
	EndgameLeadPerCard    Score
	UncertaintyMaxPenalty Score
}

// DefaultWeights returns the first explainable, non-learned heuristic weights.
func DefaultWeights() Weights {
	return Weights{
		MaterialPerCard:       55,
		TrumpPerCard:          22,
		TrumpRankScale:        4,
		DefenseCovered:        45,
		DefenseMissed:         -120,
		AttackAction:          18,
		RoleAttacker:          20,
		RoleDefender:          -20,
		TakingRisk:            -90,
		EndgameLeadPerCard:    35,
		UncertaintyMaxPenalty: 60,
	}
}

// Evaluator scores visible positions for one seat.
type Evaluator struct {
	Weights Weights
}

// DefaultEvaluator creates the first static evaluator.
func DefaultEvaluator() Evaluator {
	return Evaluator{Weights: DefaultWeights()}
}

// Evaluate scores a position with default weights.
func Evaluate(decision *app.DecisionContext, hidden HiddenCards) PositionEvaluation {
	evaluator := DefaultEvaluator()
	return evaluator.Evaluate(decision, hidden)
}

// Evaluate scores a position with configured weights.
func (e *Evaluator) Evaluate(decision *app.DecisionContext, hidden HiddenCards) PositionEvaluation {
	if decision == nil {
		return NewPositionEvaluation(domain.NoSeat, 0)
	}
	if e.Weights == (Weights{}) {
		e.Weights = DefaultWeights()
	}
	hidden = ensureHiddenCards(decision, hidden)
	confidence := hiddenConfidence(hidden)
	if decision.Phase == domain.MatchPhaseComplete && decision.Winner == decision.Seat {
		return NewPositionEvaluation(decision.Seat, 100, FeatureContribution{
			Name:   FeatureTerminal,
			Score:  MaxScore,
			Reason: "evaluated seat has already won",
		})
	}
	if decision.Phase == domain.MatchPhaseComplete && decision.Loser == decision.Seat {
		return NewPositionEvaluation(decision.Seat, 100, FeatureContribution{
			Name:   FeatureTerminal,
			Score:  MinScore,
			Reason: "evaluated seat has already lost",
		})
	}

	features := []FeatureContribution{
		e.materialPressure(decision),
		e.trumpStrength(decision),
		e.defenseCoverage(decision),
		e.attackPressure(decision),
		e.rolePhaseRisk(decision),
		e.endgamePressure(decision),
		e.uncertaintyPenalty(confidence),
	}
	return NewPositionEvaluation(decision.Seat, confidence, features...)
}

func (e *Evaluator) materialPressure(decision *app.DecisionContext) FeatureContribution {
	lead, ok := visibleHandLead(decision)
	if !ok {
		return FeatureContribution{Name: FeatureMaterialPressure}
	}
	return FeatureContribution{
		Name:   FeatureMaterialPressure,
		Score:  Score(lead) * e.Weights.MaterialPerCard,
		Reason: fmt.Sprintf("visible hand lead %d cards", lead),
	}
}

func (e *Evaluator) trumpStrength(decision *app.DecisionContext) FeatureContribution {
	var score Score
	trumps := 0
	for _, card := range decision.Hand {
		if card.Suit != decision.TrumpSuit {
			continue
		}
		trumps++
		score += e.Weights.TrumpPerCard + Score(card.Rank-domain.Six)*e.Weights.TrumpRankScale
	}
	return FeatureContribution{
		Name:   FeatureTrumpStrength,
		Score:  score,
		Reason: fmt.Sprintf("%d trumps in hand", trumps),
	}
}

func (e *Evaluator) defenseCoverage(decision *app.DecisionContext) FeatureContribution {
	if decision.Seat != decision.Defender || decision.Phase != domain.MatchPhaseDefense {
		return FeatureContribution{Name: FeatureDefenseCoverage}
	}
	var score Score
	pending := 0
	covered := 0
	for _, pair := range decision.Table {
		if pair.Defended {
			continue
		}
		pending++
		if _, ok := cheapestDefenseCard(pair.Attack, decision.Hand, decision.TrumpSuit); ok {
			covered++
			score += e.Weights.DefenseCovered
			continue
		}
		score += e.Weights.DefenseMissed
	}
	return FeatureContribution{
		Name:   FeatureDefenseCoverage,
		Score:  score,
		Reason: fmt.Sprintf("%d/%d pending attacks covered", covered, pending),
	}
}

func (e *Evaluator) attackPressure(decision *app.DecisionContext) FeatureContribution {
	actions := 0
	for _, action := range decision.LegalActions {
		switch action.Kind {
		case domain.ActionKindAttack, domain.ActionKindThrowIn, domain.ActionKindTransfer:
			actions++
		}
	}
	return FeatureContribution{
		Name:   FeatureAttackPressure,
		Score:  Score(actions) * e.Weights.AttackAction,
		Reason: fmt.Sprintf("%d active attack actions", actions),
	}
}

func (e *Evaluator) rolePhaseRisk(decision *app.DecisionContext) FeatureContribution {
	var score Score
	switch {
	case decision.Phase == domain.MatchPhaseUnknown || decision.Phase == domain.MatchPhaseComplete:
		score = 0
	case decision.Phase == domain.MatchPhaseTaking && decision.Seat == decision.Defender:
		score += e.Weights.TakingRisk
	case decision.Seat == decision.Attacker:
		score += e.Weights.RoleAttacker
	case decision.Seat == decision.Defender:
		score += e.Weights.RoleDefender
	}
	return FeatureContribution{Name: FeatureRolePhaseRisk, Score: score}
}

func (e *Evaluator) endgamePressure(decision *app.DecisionContext) FeatureContribution {
	if decision.StockCount > 3 {
		return FeatureContribution{Name: FeatureEndgamePressure}
	}
	lead, ok := visibleHandLead(decision)
	if !ok {
		return FeatureContribution{Name: FeatureEndgamePressure}
	}
	multiplier := Score(1)
	if decision.StockCount == 0 {
		multiplier = 2
	}
	return FeatureContribution{
		Name:   FeatureEndgamePressure,
		Score:  Score(lead) * e.Weights.EndgameLeadPerCard * multiplier,
		Reason: fmt.Sprintf("stock count %d", decision.StockCount),
	}
}

func (e *Evaluator) uncertaintyPenalty(confidence int) FeatureContribution {
	penalty := -Score(100-confidence) * e.Weights.UncertaintyMaxPenalty / 100
	return FeatureContribution{
		Name:   FeatureUncertaintyPenalty,
		Score:  penalty,
		Reason: fmt.Sprintf("confidence %d", confidence),
	}
}

func visibleHandLead(decision *app.DecisionContext) (int, bool) {
	if decision == nil {
		return 0, false
	}
	seat := int(decision.Seat)
	if seat < 0 || seat >= len(decision.HandSizes) {
		return 0, false
	}
	ownSize := len(decision.Hand)
	totalOpponents := 0
	opponents := 0
	for index, size := range decision.HandSizes {
		if index == seat {
			continue
		}
		totalOpponents += size
		opponents++
	}
	if opponents == 0 {
		return 0, false
	}
	return totalOpponents/opponents - ownSize, true
}

func cheapestDefenseCard(
	attack domain.Card,
	hand []domain.Card,
	trumpSuit domain.Suit,
) (domain.Card, bool) {
	var best domain.Card
	found := false
	for _, card := range hand {
		if !domain.CanBeat(attack, card, trumpSuit) {
			continue
		}
		if !found || cardCost(card, trumpSuit) < cardCost(best, trumpSuit) {
			best = card
			found = true
		}
	}
	return best, found
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

func cardCost(card domain.Card, trumpSuit domain.Suit) int {
	cost := int(card.Rank)
	if card.Suit == trumpSuit {
		cost += 20
	}
	return cost
}
