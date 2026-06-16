package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestEvaluateMaterialLeadIsPositive(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{2, 6},
			StockCount: 10,
		},
		Hand: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Seven, domain.Diamonds),
		},
	}

	result := evaluation.Evaluate(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if result.Score <= 0 {
		t.Fatalf("Score = %d, want positive material lead", result.Score)
	}
	if featureScore(result, evaluation.FeatureMaterialPressure) <= 0 {
		t.Fatalf("material feature = %d, want positive", featureScore(result, evaluation.FeatureMaterialPressure))
	}
}

func TestEvaluateWeakDefenseCoverageIsNegative(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(1),
			Phase:      domain.MatchPhaseDefense,
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{2, 2},
			StockCount: 10,
			Table: []domain.TablePair{
				{Attack: card(domain.Ace, domain.Spades)},
			},
		},
		Hand: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Seven, domain.Diamonds),
		},
	}

	result := evaluation.Evaluate(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if result.Score >= 0 {
		t.Fatalf("Score = %d, want negative weak defense", result.Score)
	}
	if featureScore(result, evaluation.FeatureDefenseCoverage) >= 0 {
		t.Fatalf("defense feature = %d, want negative", featureScore(result, evaluation.FeatureDefenseCoverage))
	}
}

func TestEvaluateHighTrumpsArePositive(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{4, 4},
			StockCount: 10,
		},
		Hand: []domain.Card{
			card(domain.Ace, domain.Hearts),
			card(domain.King, domain.Hearts),
			card(domain.Queen, domain.Hearts),
			card(domain.Six, domain.Clubs),
		},
	}

	result := evaluation.Evaluate(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if result.Score <= 0 {
		t.Fatalf("Score = %d, want positive trump strength", result.Score)
	}
	if featureScore(result, evaluation.FeatureTrumpStrength) <= 0 {
		t.Fatalf("trump feature = %d, want positive", featureScore(result, evaluation.FeatureTrumpStrength))
	}
}

func TestEvaluateConfidenceDropsWithLargeUnknownPool(t *testing.T) {
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:           domain.Seat(0),
			TrumpSuit:      domain.Hearts,
			TrumpIndicator: card(domain.Ace, domain.Hearts),
			HandSizes:      []int{6, 6},
			StockCount:     23,
		},
		Hand: []domain.Card{
			card(domain.Six, domain.Clubs),
			card(domain.Seven, domain.Clubs),
			card(domain.Eight, domain.Clubs),
			card(domain.Nine, domain.Clubs),
			card(domain.Ten, domain.Clubs),
			card(domain.Jack, domain.Clubs),
		},
	}

	result := evaluation.Evaluate(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if result.Confidence >= 60 {
		t.Fatalf("Confidence = %d, want low confidence under 60", result.Confidence)
	}
	if featureScore(result, evaluation.FeatureUncertaintyPenalty) >= 0 {
		t.Fatalf("uncertainty feature = %d, want negative", featureScore(result, evaluation.FeatureUncertaintyPenalty))
	}
}

func featureScore(result evaluation.PositionEvaluation, name string) evaluation.Score {
	for _, feature := range result.Features {
		if feature.Name == name {
			return feature.Score
		}
	}
	return 0
}
