package evaluation

import (
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

const (
	// FeatureTerminal explains already completed wins or losses.
	FeatureTerminal = "terminal"
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
