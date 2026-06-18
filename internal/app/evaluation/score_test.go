package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app/evaluation"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name  string
		score evaluation.Score
		want  evaluation.Score
	}{
		{name: "lower bound", score: -1200, want: evaluation.MinScore},
		{name: "keeps middle", score: 125, want: 125},
		{name: "upper bound", score: 1200, want: evaluation.MaxScore},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := evaluation.Clamp(test.score); got != test.want {
				t.Fatalf("Clamp(%d) = %d, want %d", test.score, got, test.want)
			}
		})
	}
}

func TestScoreFromDurakProbability(t *testing.T) {
	tests := []struct {
		name          string
		probability   float64
		activePlayers int
		want          evaluation.Score
	}{
		{name: "two-player win", probability: 0, activePlayers: 2, want: evaluation.MaxScore},
		{name: "two-player neutral", probability: 0.5, activePlayers: 2, want: 0},
		{name: "two-player loss", probability: 1, activePlayers: 2, want: evaluation.MinScore},
		{name: "six-player neutral", probability: 1.0 / 6.0, activePlayers: 6, want: 0},
		{name: "single active player already safe", probability: 1, activePlayers: 1, want: evaluation.MaxScore},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := evaluation.ScoreFromDurakProbability(test.probability, test.activePlayers)
			if got != test.want {
				t.Fatalf("ScoreFromDurakProbability(%v, %d) = %d, want %d",
					test.probability, test.activePlayers, got, test.want)
			}
		})
	}
}

func TestQualityFromLoss(t *testing.T) {
	tests := []struct {
		loss evaluation.Score
		want evaluation.MoveQuality
	}{
		{loss: -10, want: evaluation.MoveQualityBest},
		{loss: 0, want: evaluation.MoveQualityBest},
		{loss: 20, want: evaluation.MoveQualityBest},
		{loss: 21, want: evaluation.MoveQualityGood},
		{loss: 80, want: evaluation.MoveQualityGood},
		{loss: 81, want: evaluation.MoveQualityInaccuracy},
		{loss: 180, want: evaluation.MoveQualityInaccuracy},
		{loss: 181, want: evaluation.MoveQualityMistake},
		{loss: 350, want: evaluation.MoveQualityMistake},
		{loss: 351, want: evaluation.MoveQualityBlunder},
	}

	for _, test := range tests {
		if got := evaluation.QualityFromLoss(test.loss); got != test.want {
			t.Fatalf("QualityFromLoss(%d) = %s, want %s", test.loss, got, test.want)
		}
	}
}
