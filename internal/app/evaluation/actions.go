package evaluation

import (
	"math"
	"slices"
	"sort"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// ActionEvaluation is a ranked legal action for the active seat.
type ActionEvaluation struct {
	Action   domain.Action
	Score    Score
	Delta    Score
	Loss     Score
	Quality  MoveQuality
	Features []FeatureContribution
}

// RankActions scores legal actions by projected risk after the local battle.
func RankActions(decision *app.DecisionContext, hidden HiddenCards) []ActionEvaluation {
	if decision == nil || len(decision.LegalActions) == 0 {
		return nil
	}
	hidden = ensureHiddenCards(decision, hidden)
	base := Evaluate(decision, hidden)
	results := make([]ActionEvaluation, 0, len(decision.LegalActions))
	for _, action := range decision.LegalActions {
		score := ScoreAction(decision, hidden, action)
		results = append(results, ActionEvaluation{
			Action: action,
			Score:  score,
			Delta:  score - base.Score,
			Features: []FeatureContribution{{
				Name:  FeatureRiskScore,
				Score: score,
			}},
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	bestScore := results[0].Score
	for index := range results {
		results[index].Loss = bestScore - results[index].Score
		results[index].Quality = QualityFromLoss(results[index].Loss)
	}
	return results
}

// ScoreAction projects one legal action and scores the resulting fair position.
func ScoreAction(decision *app.DecisionContext, hidden HiddenCards, action domain.Action) Score {
	if decision == nil {
		return 0
	}
	hidden = ensureHiddenCards(decision, hidden)
	switch action.Kind {
	case domain.ActionKindAttack, domain.ActionKindThrowIn:
		return scoreAttackLikeAction(decision, hidden, action)
	default:
		projected := projectAction(decision, action)
		score := Evaluate(&projected, BuildHiddenCards(&projected, nil)).Score
		return Clamp(score - actionResourcePenalty(decision, action))
	}
}

func scoreAttackLikeAction(decision *app.DecisionContext, hidden HiddenCards, action domain.Action) Score {
	added := projectAction(decision, action)
	if decision.Phase == domain.MatchPhaseTaking {
		taken := added
		projectFinishTake(&taken)
		return Clamp(Evaluate(&taken, BuildHiddenCards(&taken, nil)).Score - actionResourcePenalty(decision, action))
	}
	pending := pendingAttacks(added.Table)
	if len(pending) == 0 {
		return Evaluate(&added, BuildHiddenCards(&added, nil)).Score
	}
	beat := added
	for index := range beat.Table {
		if !beat.Table[index].Defended {
			beat.Table[index].Defended = true
		}
	}
	changeHandSize(&beat, beat.Defender, -len(pending))
	beat.Phase = domain.MatchPhaseThrowIn
	take := added
	projectFinishTake(&take)
	pCover := CoverProbability(
		pending,
		added.Defender,
		visibleHandSize(&added, added.Defender, 0),
		added.TrumpSuit,
		hidden,
	)
	beatScore := float64(Evaluate(&beat, BuildHiddenCards(&beat, nil)).Score)
	takeScore := float64(Evaluate(&take, BuildHiddenCards(&take, nil)).Score)
	score := Score(math.Round(pCover*beatScore + (1-pCover)*takeScore))
	return Clamp(score - actionResourcePenalty(decision, action))
}

func actionResourcePenalty(decision *app.DecisionContext, action domain.Action) Score {
	switch action.Kind {
	case domain.ActionKindAttack, domain.ActionKindThrowIn:
		penalty := cardStickiness(action.Card, decision.TrumpSuit, stockFinality(decision, 0.10)) * 115
		if action.Card.Suit == decision.TrumpSuit {
			penalty += 160
		}
		return Score(math.Round(penalty))
	case domain.ActionKindDefend:
		return Score(math.Round(defenseSpendCost(action.Card, decision) * 55))
	case domain.ActionKindTransfer:
		return Score(math.Round((defenseSpendCost(action.Card, decision) + transferRiskCost(decision, action.Card)) * 180))
	case domain.ActionKindTake:
		risk := EvaluateBattleRisk(decision, BuildHiddenCards(decision, nil))
		if risk.Best < risk.TakeNow {
			return Score(math.Round(90 + (risk.TakeNow-risk.Best)*360))
		}
		return 0
	default:
		return 0
	}
}

func projectAction(decision *app.DecisionContext, action domain.Action) app.DecisionContext {
	projected := cloneDecisionForAction(decision)
	switch action.Kind {
	case domain.ActionKindAttack:
		removeProjectedCard(&projected, action.Seat, action.Card)
		projected.Table = append(projected.Table, domain.TablePair{Attack: action.Card})
		projected.Attacker = action.Seat
		projected.Phase = domain.MatchPhaseDefense
	case domain.ActionKindThrowIn:
		removeProjectedCard(&projected, action.Seat, action.Card)
		projected.Table = append(projected.Table, domain.TablePair{Attack: action.Card})
		if projected.Phase == domain.MatchPhaseThrowIn {
			projected.Phase = domain.MatchPhaseDefense
		}
	case domain.ActionKindDefend:
		removeProjectedCard(&projected, action.Seat, action.Card)
		if action.AttackIndex >= 0 && action.AttackIndex < len(projected.Table) {
			projected.Table[action.AttackIndex].Defense = action.Card
			projected.Table[action.AttackIndex].Defended = true
		}
		if allProjectedAttacksDefended(projected.Table) {
			projected.Phase = domain.MatchPhaseThrowIn
		}
	case domain.ActionKindTransfer:
		removeProjectedCard(&projected, action.Seat, action.Card)
		projected.Table = append(projected.Table, domain.TablePair{Attack: action.Card})
		projected.Attacker = action.Seat
		projected.Defender = nextProjectedSeat(&projected, action.Seat)
		projected.Phase = domain.MatchPhaseDefense
	case domain.ActionKindTake, domain.ActionKindFinishTake:
		projectFinishTake(&projected)
	case domain.ActionKindFinishDefense:
		projectFinishDefense(&projected)
	case domain.ActionKindPassThrowIn:
	}
	return projected
}

func cloneDecisionForAction(decision *app.DecisionContext) app.DecisionContext {
	if decision == nil {
		return app.DecisionContext{}
	}
	projected := *decision
	projected.Table = slices.Clone(decision.Table)
	projected.Hand = slices.Clone(decision.Hand)
	projected.HandSizes = slices.Clone(decision.HandSizes)
	projected.LegalActions = nil
	projected.PublicMemory = app.PublicCardMemory{}
	return projected
}

func projectFinishDefense(projected *app.DecisionContext) {
	oldDefender := projected.Defender
	projected.DiscardCount += len(tableCardsForRisk(projected.Table))
	projected.Table = nil
	projectRefill(projected, projected.Attacker, oldDefender)
	if completeProjectedIfFinished(projected) {
		return
	}
	projected.Attacker = activeProjectedSeatFrom(projected, oldDefender)
	projected.Defender = nextProjectedSeat(projected, projected.Attacker)
	projected.Phase = domain.MatchPhaseAttack
}

func projectFinishTake(projected *app.DecisionContext) {
	oldDefender := projected.Defender
	cards := tableCardsForRisk(projected.Table)
	changeHandSize(projected, oldDefender, len(cards))
	if projected.Seat == oldDefender {
		projected.Hand = append(projected.Hand, cards...)
	}
	projected.Table = nil
	projectRefill(projected, projected.Attacker, oldDefender)
	if completeProjectedIfFinished(projected) {
		return
	}
	start := nextProjectedSeat(projected, oldDefender)
	projected.Attacker = activeProjectedSeatFrom(projected, start)
	projected.Defender = nextProjectedSeat(projected, projected.Attacker)
	projected.Phase = domain.MatchPhaseAttack
}

func projectRefill(projected *app.DecisionContext, order ...domain.Seat) {
	seen := make(map[domain.Seat]bool, len(order))
	for _, seat := range order {
		if seen[seat] || !validProjectedSeat(projected, seat) {
			continue
		}
		seen[seat] = true
		need := 6 - projected.HandSizes[int(seat)]
		if need <= 0 {
			continue
		}
		drawn := min(need, projected.StockCount)
		projected.HandSizes[int(seat)] += drawn
		projected.StockCount -= drawn
		if projected.Seat == seat {
			projected.Hand = append(projected.Hand, make([]domain.Card, drawn)...)
		}
	}
}

func removeProjectedCard(projected *app.DecisionContext, seat domain.Seat, card domain.Card) {
	changeHandSize(projected, seat, -1)
	if projected.Seat != seat {
		return
	}
	index := slices.Index(projected.Hand, card)
	if index >= 0 {
		projected.Hand = slices.Delete(projected.Hand, index, index+1)
	}
}

func changeHandSize(projected *app.DecisionContext, seat domain.Seat, delta int) {
	if !validProjectedSeat(projected, seat) {
		return
	}
	next := projected.HandSizes[int(seat)] + delta
	if next < 0 {
		next = 0
	}
	projected.HandSizes[int(seat)] = next
}

func completeProjectedIfFinished(projected *app.DecisionContext) bool {
	if projected.StockCount > 0 || len(projected.Table) > 0 {
		return false
	}
	active := make([]domain.Seat, 0, len(projected.HandSizes))
	for seat, size := range projected.HandSizes {
		if size > 0 {
			active = append(active, domain.Seat(seat))
		}
	}
	if len(active) > 1 {
		return false
	}
	projected.Phase = domain.MatchPhaseComplete
	projected.Attacker = domain.NoSeat
	projected.Defender = domain.NoSeat
	if len(active) == 1 {
		projected.Loser = active[0]
		projected.Winner = firstProjectedEmptySeat(projected)
	}
	return true
}

func allProjectedAttacksDefended(table []domain.TablePair) bool {
	if len(table) == 0 {
		return false
	}
	for _, pair := range table {
		if !pair.Defended {
			return false
		}
	}
	return true
}

func nextProjectedSeat(projected *app.DecisionContext, seat domain.Seat) domain.Seat {
	if len(projected.HandSizes) < 2 {
		return domain.NoSeat
	}
	start := int(seat)
	for offset := 1; offset <= len(projected.HandSizes); offset++ {
		next := domain.Seat((start + offset) % len(projected.HandSizes))
		if activeProjectedSeat(projected, next) {
			return next
		}
	}
	return domain.NoSeat
}

func activeProjectedSeatFrom(projected *app.DecisionContext, start domain.Seat) domain.Seat {
	if len(projected.HandSizes) == 0 {
		return domain.NoSeat
	}
	startIndex := int(start)
	if startIndex < 0 {
		startIndex = 0
	}
	for offset := range projected.HandSizes {
		seat := domain.Seat((startIndex + offset) % len(projected.HandSizes))
		if activeProjectedSeat(projected, seat) {
			return seat
		}
	}
	return domain.NoSeat
}

func activeProjectedSeat(projected *app.DecisionContext, seat domain.Seat) bool {
	if !validProjectedSeat(projected, seat) {
		return false
	}
	return projected.StockCount > 0 || len(projected.Table) > 0 || projected.HandSizes[int(seat)] > 0
}

func validProjectedSeat(projected *app.DecisionContext, seat domain.Seat) bool {
	return seat != domain.NoSeat && int(seat) >= 0 && int(seat) < len(projected.HandSizes)
}

func firstProjectedEmptySeat(projected *app.DecisionContext) domain.Seat {
	for seat, size := range projected.HandSizes {
		if size == 0 {
			return domain.Seat(seat)
		}
	}
	return domain.NoSeat
}
