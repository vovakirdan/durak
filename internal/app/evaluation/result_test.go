package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestNewPositionEvaluationSumsAndClamps(t *testing.T) {
	result := evaluation.NewPositionEvaluation(
		domain.Seat(1),
		150,
		evaluation.FeatureContribution{Name: evaluation.FeatureRiskHandBurden, Score: 900},
		evaluation.FeatureContribution{Name: evaluation.FeatureRiskOutlet, Score: 250},
	)

	if result.Seat != domain.Seat(1) {
		t.Fatalf("Seat = %d, want 1", result.Seat)
	}
	if result.Score != evaluation.MaxScore {
		t.Fatalf("Score = %d, want %d", result.Score, evaluation.MaxScore)
	}
	if result.Confidence != 100 {
		t.Fatalf("Confidence = %d, want 100", result.Confidence)
	}
	if len(result.Features) != 2 {
		t.Fatalf("Features length = %d, want 2", len(result.Features))
	}
}

func TestNewPositionEvaluationClampsNegativeConfidence(t *testing.T) {
	result := evaluation.NewPositionEvaluation(
		domain.Seat(0),
		-10,
		evaluation.FeatureContribution{Name: evaluation.FeatureRiskScore, Score: -1200},
	)

	if result.Score != evaluation.MinScore {
		t.Fatalf("Score = %d, want %d", result.Score, evaluation.MinScore)
	}
	if result.Confidence != 0 {
		t.Fatalf("Confidence = %d, want 0", result.Confidence)
	}
}
