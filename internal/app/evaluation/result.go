package evaluation

import (
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

const (
	// FeatureTerminal explains already completed wins or losses.
	FeatureTerminal = "terminal"
	// FeatureMaterialPressure compares visible hand sizes.
	FeatureMaterialPressure = "material_pressure"
	// FeatureTrumpStrength evaluates trump count and rank quality.
	FeatureTrumpStrength = "trump_strength"
	// FeatureDefenseCoverage evaluates whether current attacks can be beaten.
	FeatureDefenseCoverage = "defense_coverage"
	// FeatureAttackPressure evaluates available attacks, throws, and transfers.
	FeatureAttackPressure = "attack_pressure"
	// FeatureRolePhaseRisk captures attacker, defender, and taking-phase pressure.
	FeatureRolePhaseRisk = "role_phase_risk"
	// FeatureEndgamePressure adds weight when stock is low or empty.
	FeatureEndgamePressure = "endgame_pressure"
	// FeatureUncertaintyPenalty lowers score confidence when many cards are unknown.
	FeatureUncertaintyPenalty = "uncertainty_penalty"
	// FeatureActionKind explains the baseline action-type delta.
	FeatureActionKind = "action_kind"
	// FeatureActionCardEconomy explains card spending quality for an action.
	FeatureActionCardEconomy = "action_card_economy"
	// FeatureActionPressure explains tactical pressure created by an action.
	FeatureActionPressure = "action_pressure"
	// FeatureActionDefenseSafety explains risks left after a defense action.
	FeatureActionDefenseSafety = "action_defense_safety"
)

// FeatureContribution explains one part of a position or action score.
type FeatureContribution struct {
	Name   string
	Score  Score
	Reason string
}

// PositionEvaluation is the evaluated position for one visible seat view.
type PositionEvaluation struct {
	Seat       domain.Seat
	Score      Score
	Confidence int
	Features   []FeatureContribution
}

// NewPositionEvaluation sums feature scores and clamps public result fields.
func NewPositionEvaluation(
	seat domain.Seat,
	confidence int,
	features ...FeatureContribution,
) PositionEvaluation {
	var score Score
	for _, feature := range features {
		score += feature.Score
	}
	return PositionEvaluation{
		Seat:       seat,
		Score:      Clamp(score),
		Confidence: ClampConfidence(confidence),
		Features:   slices.Clone(features),
	}
}

// ClampConfidence keeps confidence as a percentage-like public value.
func ClampConfidence(confidence int) int {
	if confidence < 0 {
		return 0
	}
	if confidence > 100 {
		return 100
	}
	return confidence
}
