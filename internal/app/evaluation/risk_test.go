package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRiskModelFewerCardsIsBetterWhenStockEmpty(t *testing.T) {
	low := riskDecision([]int{2, 6}, 0)
	high := riskDecision([]int{6, 2}, 0)
	model := evaluation.DefaultRiskModel()

	lowScore := model.Evaluate(&low, evaluation.BuildHiddenCards(&low, nil)).Score
	highScore := model.Evaluate(&high, evaluation.BuildHiddenCards(&high, nil)).Score

	if lowScore <= highScore {
		t.Fatalf("low score = %d, high score = %d; fewer cards with empty stock should score higher",
			lowScore, highScore)
	}
}

func TestRiskModelFullStockDoesNotOverrewardShortHand(t *testing.T) {
	low := riskDecision([]int{2, 6}, 20)
	normal := riskDecision([]int{6, 6}, 20)
	model := evaluation.DefaultRiskModel()

	lowScore := model.Evaluate(&low, evaluation.BuildHiddenCards(&low, nil)).Score
	normalScore := model.Evaluate(&normal, evaluation.BuildHiddenCards(&normal, nil)).Score

	if lowScore-normalScore > 250 {
		t.Fatalf("low score = %d, normal score = %d; short hand with full stock is overrewarded",
			lowScore, normalScore)
	}
}

func TestEvaluateUsesRiskModel(t *testing.T) {
	decision := riskDecision([]int{2, 6}, 0)

	result := evaluation.Evaluate(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if result.Score <= 0 {
		t.Fatalf("Score = %d, want positive risk score", result.Score)
	}
	if riskFeatureScore(result) != result.Score {
		t.Fatalf("risk feature = %d, want score %d",
			riskFeatureScore(result), result.Score)
	}
}

func riskDecision(handSizes []int, stock int) app.DecisionContext {
	seat := domain.Seat(0)
	hand := make([]domain.Card, handSizes[int(seat)])
	for index := range hand {
		hand[index] = card(domain.Rank(int(domain.Six)+index%int(domain.Ace-domain.Six+1)), domain.Clubs)
	}
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       seat,
			Phase:      domain.MatchPhaseAttack,
			Attacker:   seat,
			Defender:   domain.Seat((int(seat) + 1) % len(handSizes)),
			TrumpSuit:  domain.Hearts,
			HandSizes:  handSizes,
			StockCount: stock,
		},
		Hand: hand,
	}
}
