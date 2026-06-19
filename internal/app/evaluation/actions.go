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
	resolution := ResolveBattleExpected(decision, hidden, action)
	score := Evaluate(&resolution.Context, BuildHiddenCards(&resolution.Context, nil)).Score
	penalty := localActionRiskCost(decision, hidden, action) * localActionRiskScale(action)
	penalty += helpfulPickupRisk(decision, action, &resolution) * 60
	return Clamp(score - Score(math.Round(penalty)))
}

func localActionRiskScale(action domain.Action) float64 {
	switch action.Kind {
	case domain.ActionKindDefend, domain.ActionKindTransfer:
		return 75
	case domain.ActionKindAttack, domain.ActionKindThrowIn:
		return 20
	default:
		return 0
	}
}

func localActionRiskCost(decision *app.DecisionContext, hidden HiddenCards, action domain.Action) float64 {
	switch action.Kind {
	case domain.ActionKindAttack:
		cost := 0.0
		for _, card := range action.AttackCards() {
			cost += releaseCost(card, decision, action.Seat)
		}
		return cost
	case domain.ActionKindThrowIn:
		return releaseCost(action.Card, decision, action.Seat)
	case domain.ActionKindDefend:
		return DefenseActionRisk(decision, hidden, action)
	case domain.ActionKindTransfer:
		return transferCardCost(action.Card, decision)
	default:
		return 0
	}
}

func releaseCost(card domain.Card, decision *app.DecisionContext, seat domain.Seat) float64 {
	if !validCard(card) {
		return 1
	}
	cost := baseCardCost(card) + cardStickinessForSeat(card, decision, seat, decision.Hand)
	if card.Suit == decision.TrumpSuit {
		cost += defaultTrumpPremium * (1 - endgameFactor(decision)*0.5)
	}
	return cost
}

func helpfulPickupRisk(
	decision *app.DecisionContext,
	action domain.Action,
	resolution *BattleResolution,
) float64 {
	if decision == nil || resolution == nil || resolution.FirstResponse != BattleBranchTake ||
		resolution.Context.Winner == decision.Seat {
		return 0
	}
	switch action.Kind {
	case domain.ActionKindAttack:
		risk := 0.0
		for _, card := range action.AttackCards() {
			risk += helpfulPickupCardRisk(card, decision)
		}
		return risk
	case domain.ActionKindThrowIn:
		return helpfulPickupCardRisk(action.Card, decision)
	default:
		return 0
	}
}

func helpfulPickupCardRisk(card domain.Card, decision *app.DecisionContext) float64 {
	if !validCard(card) {
		return 0
	}
	risk := 0.0
	if card.Suit == decision.TrumpSuit {
		risk += 2.0
	} else if rankValue(card) >= 5 {
		risk += 1.0
	}
	if rankValue(card) <= 2 {
		risk += 0.7
	}
	return risk * endgameFactor(decision)
}

func projectAction(decision *app.DecisionContext, action domain.Action) app.DecisionContext {
	projected := cloneDecisionForAction(decision)
	switch action.Kind {
	case domain.ActionKindAttack:
		for _, card := range action.AttackCards() {
			removeProjectedCard(&projected, action.Seat, card)
			projected.Table = append(projected.Table, domain.TablePair{Attack: card})
		}
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
	projected.PublicMemory = decision.PublicMemory.Clone()
	syncProjectedMemory(&projected)
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
	rememberProjectedKnownHeld(projected, oldDefender, cards)
	projected.Table = nil
	projectRefill(projected, projected.Attacker, oldDefender)
	syncProjectedMemory(projected)
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
	}
}

func removeProjectedCard(projected *app.DecisionContext, seat domain.Seat, card domain.Card) {
	changeHandSize(projected, seat, -1)
	if projected.Seat != seat {
		removeProjectedMemoryCard(projected, seat, card)
		return
	}
	index := slices.Index(projected.Hand, card)
	if index >= 0 {
		projected.Hand = slices.Delete(projected.Hand, index, index+1)
	}
	removeProjectedMemoryCard(projected, seat, card)
	syncProjectedMemory(projected)
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

func rememberProjectedKnownHeld(projected *app.DecisionContext, seat domain.Seat, cards []domain.Card) {
	if projected == nil || !validProjectedSeat(projected, seat) {
		return
	}
	ensureProjectedMemorySeats(&projected.PublicMemory, len(projected.HandSizes))
	projected.PublicMemory.KnownHeld[int(seat)] = appendKnownCards(
		projected.PublicMemory.KnownHeld[int(seat)],
		cards...,
	)
	projected.PublicMemory.Seen = appendKnownCards(projected.PublicMemory.Seen, cards...)
}

func syncProjectedMemory(projected *app.DecisionContext) {
	if projected == nil {
		return
	}
	memory := &projected.PublicMemory
	memory.Seat = projected.Seat
	memory.Hand = slices.Clone(projected.Hand)
	memory.Table = slices.Clone(projected.Table)
	memory.HandSizes = slices.Clone(projected.HandSizes)
	memory.StockCount = projected.StockCount
	memory.TrumpIndicator = projected.TrumpIndicator
	memory.TrumpSuit = projected.TrumpSuit
	ensureProjectedMemorySeats(memory, len(projected.HandSizes))
	if validProjectedSeat(projected, projected.Seat) {
		memory.KnownHeld[int(projected.Seat)] = slices.Clone(projected.Hand)
	}
	memory.Seen = appendKnownCards(memory.Seen, projected.Hand...)
	memory.Seen = appendKnownCards(memory.Seen, projected.TrumpIndicator)
	for _, pair := range projected.Table {
		memory.Seen = appendKnownCards(memory.Seen, pair.Attack)
		if pair.Defended {
			memory.Seen = appendKnownCards(memory.Seen, pair.Defense)
		}
	}
}

func removeProjectedMemoryCard(projected *app.DecisionContext, seat domain.Seat, card domain.Card) {
	if projected == nil || !validProjectedSeat(projected, seat) {
		return
	}
	memory := &projected.PublicMemory
	ensureProjectedMemorySeats(memory, len(projected.HandSizes))
	memory.KnownHeld[int(seat)] = removeMemoryCard(memory.KnownHeld[int(seat)], card)
	memory.Seen = appendKnownCards(memory.Seen, card)
}

func ensureProjectedMemorySeats(memory *app.PublicCardMemory, count int) {
	if count <= len(memory.KnownHeld) {
		return
	}
	memory.KnownHeld = append(memory.KnownHeld, make([][]domain.Card, count-len(memory.KnownHeld))...)
}

func removeMemoryCard(cards []domain.Card, card domain.Card) []domain.Card {
	index := slices.Index(cards, card)
	if index < 0 {
		return cards
	}
	return slices.Delete(cards, index, index+1)
}
