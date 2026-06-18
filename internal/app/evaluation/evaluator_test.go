package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestEvaluateReturnsTerminalWinAndLoss(t *testing.T) {
	win := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:   domain.Seat(0),
			Phase:  domain.MatchPhaseComplete,
			Winner: domain.Seat(0),
			Loser:  domain.Seat(1),
		},
	}
	loss := win
	loss.Seat = domain.Seat(1)

	winResult := evaluation.Evaluate(&win, evaluation.HiddenCards{})
	lossResult := evaluation.Evaluate(&loss, evaluation.HiddenCards{})

	if winResult.Score != evaluation.MaxScore {
		t.Fatalf("win score = %d, want %d", winResult.Score, evaluation.MaxScore)
	}
	if lossResult.Score != evaluation.MinScore {
		t.Fatalf("loss score = %d, want %d", lossResult.Score, evaluation.MinScore)
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
	if riskFeatureScore(result) != result.Score {
		t.Fatalf("risk feature = %d, want score %d",
			riskFeatureScore(result), result.Score)
	}
}

func riskFeatureScore(result evaluation.PositionEvaluation) evaluation.Score {
	for _, feature := range result.Features {
		if feature.Name == evaluation.FeatureRiskScore {
			return feature.Score
		}
	}
	return 0
}
