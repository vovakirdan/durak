package evaluation_test

import (
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/evaluation"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRankActionsDefendsBeforeTaking(t *testing.T) {
	defend := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Seven, domain.Clubs),
		AttackIndex: 0,
	}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := defenseDecision([]domain.Action{take, defend})

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if results[0].Action != defend {
		t.Fatalf("best action = %+v, want defend", results[0].Action)
	}
	if results[len(results)-1].Action != take {
		t.Fatalf("worst action = %+v, want take", results[len(results)-1].Action)
	}
}

func TestRankActionsAvoidsHighTrumpWhenNonTrumpDefenseWorks(t *testing.T) {
	cheap := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Seven, domain.Clubs),
		AttackIndex: 0,
	}
	expensive := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Ace, domain.Hearts),
		AttackIndex: 0,
	}
	decision := defenseDecision([]domain.Action{expensive, cheap})

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if results[0].Action != cheap {
		t.Fatalf("best action = %+v, want cheap non-trump defense", results[0].Action)
	}
}

func TestRankActionsKeepsCheapDefenseCloseToTransfer(t *testing.T) {
	defend := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Seven, domain.Clubs),
		AttackIndex: 0,
	}
	transfer := domain.Action{
		Kind: domain.ActionKindTransfer,
		Seat: domain.Seat(1),
		Card: card(domain.Six, domain.Diamonds),
	}
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(1),
			Phase:      domain.MatchPhaseDefense,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{3, 2},
			StockCount: 12,
			Table: []domain.TablePair{
				{Attack: card(domain.Six, domain.Clubs)},
			},
		},
		Hand:         []domain.Card{defend.Card, transfer.Card},
		LegalActions: []domain.Action{transfer, defend},
	}

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	defendResult := actionResult(t, results, defend)
	if defendResult.Loss > 100 {
		t.Fatalf("defend loss = %d in results %+v, want close alternative to transfer",
			defendResult.Loss, results)
	}
}

func TestRankActionsPrefersLowNonTrumpAttack(t *testing.T) {
	low := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card(domain.Six, domain.Clubs)}
	high := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card(domain.Ace, domain.Spades)}
	trump := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card(domain.Seven, domain.Hearts)}
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      domain.MatchPhaseAttack,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{3, 3},
			StockCount: 10,
		},
		Hand: []domain.Card{low.Card, high.Card, trump.Card},
		LegalActions: []domain.Action{
			trump,
			high,
			low,
		},
	}

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if results[0].Action != low {
		t.Fatalf("best action = %+v, want low non-trump attack", results[0].Action)
	}
}

func TestRankActionsScoresPairedAttackRankAsCloseAlternative(t *testing.T) {
	loneLow := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card(domain.Six, domain.Clubs)}
	paired := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card(domain.Eight, domain.Diamonds)}
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      domain.MatchPhaseAttack,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{3, 3},
			StockCount: 2,
		},
		Hand: []domain.Card{
			loneLow.Card,
			paired.Card,
			card(domain.Eight, domain.Spades),
		},
		LegalActions: []domain.Action{loneLow, paired},
	}

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))
	pairedResult := actionResult(t, results, paired)

	if pairedResult.Loss > 80 {
		t.Fatalf("paired attack loss = %d, want close alternative", pairedResult.Loss)
	}
}

func TestRankActionsThrowsBeforeDoneUnderPressure(t *testing.T) {
	throw := domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: card(domain.Six, domain.Clubs)}
	done := domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)}
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      domain.MatchPhaseThrowIn,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{2, 3},
			StockCount: 10,
			Table: []domain.TablePair{
				{
					Attack:   card(domain.Seven, domain.Clubs),
					Defense:  card(domain.Eight, domain.Clubs),
					Defended: true,
				},
			},
		},
		Hand:         []domain.Card{throw.Card, card(domain.Ace, domain.Spades)},
		LegalActions: []domain.Action{done, throw},
	}

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if results[0].Action != throw {
		t.Fatalf("best action = %+v, want pressure throw-in", results[0].Action)
	}
}

func TestRankActionsThrowsBeforeFinishingTake(t *testing.T) {
	throw := domain.Action{Kind: domain.ActionKindThrowIn, Seat: domain.Seat(0), Card: card(domain.Six, domain.Clubs)}
	done := domain.Action{Kind: domain.ActionKindFinishTake, Seat: domain.Seat(0)}
	decision := app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(0),
			Phase:      domain.MatchPhaseTaking,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{2, 5},
			StockCount: 8,
			Table: []domain.TablePair{
				{Attack: card(domain.Seven, domain.Clubs)},
			},
		},
		Hand:         []domain.Card{throw.Card, card(domain.Ace, domain.Spades)},
		LegalActions: []domain.Action{done, throw},
	}

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if results[0].Action != throw {
		t.Fatalf("best action = %+v, want throw into taking defender", results[0].Action)
	}
}

func TestRankActionsPenalizesTakeWhenDefenseExists(t *testing.T) {
	defend := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Seven, domain.Clubs),
		AttackIndex: 0,
	}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := defenseDecision([]domain.Action{take, defend})

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))
	takeResult := actionResult(t, results, take)

	if takeResult.Loss <= 0 {
		t.Fatalf("take loss = %d, want positive", takeResult.Loss)
	}
	if takeResult.Quality != evaluation.MoveQualityBlunder {
		t.Fatalf("take quality = %s, want blunder", takeResult.Quality)
	}
}

func TestRankActionsKeepsStableActionEvaluationShape(t *testing.T) {
	defend := domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        card(domain.Seven, domain.Clubs),
		AttackIndex: 0,
	}
	take := domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}
	decision := defenseDecision([]domain.Action{take, defend})

	results := evaluation.RankActions(&decision, evaluation.BuildHiddenCards(&decision, nil))

	if len(results) != 2 {
		t.Fatalf("results = %+v, want two ranked actions", results)
	}
	if results[0].Quality == "" {
		t.Fatalf("top result = %+v, want quality label", results[0])
	}
	if results[0].Action == (domain.Action{}) {
		t.Fatalf("top result = %+v, want action copied", results[0])
	}
}

func defenseDecision(actions []domain.Action) app.DecisionContext {
	return app.DecisionContext{
		SeatView: app.SeatView{
			Seat:       domain.Seat(1),
			Phase:      domain.MatchPhaseDefense,
			Attacker:   domain.Seat(0),
			Defender:   domain.Seat(1),
			TrumpSuit:  domain.Hearts,
			HandSizes:  []int{1, 1},
			StockCount: 10,
			Table: []domain.TablePair{
				{Attack: card(domain.Six, domain.Clubs)},
			},
		},
		Hand: []domain.Card{
			card(domain.Seven, domain.Clubs),
			card(domain.Ace, domain.Hearts),
		},
		LegalActions: actions,
	}
}

func actionResult(
	t *testing.T,
	results []evaluation.ActionEvaluation,
	action domain.Action,
) evaluation.ActionEvaluation {
	t.Helper()
	for _, result := range results {
		if result.Action == action {
			return result
		}
	}
	t.Fatalf("action %+v not found in results", action)
	return evaluation.ActionEvaluation{}
}

func card(rank domain.Rank, suit domain.Suit) domain.Card {
	return domain.Card{Rank: rank, Suit: suit}
}
