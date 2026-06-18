package evaluation

import (
	"math"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// BattleRisk compares defender options in bad-card units.
type BattleRisk struct {
	TakeNow          float64
	ContinueDefense  float64
	Transfer         float64
	Best             float64
	CoverProbability float64
}

// EvaluateBattleRisk evaluates the current defender's local battle problem.
func EvaluateBattleRisk(decision *app.DecisionContext, hidden HiddenCards) BattleRisk {
	if decision == nil || decision.Seat != decision.Defender {
		return BattleRisk{}
	}
	if decision.Phase != domain.MatchPhaseDefense && decision.Phase != domain.MatchPhaseTaking {
		return BattleRisk{}
	}
	hidden = ensureHiddenCards(decision, hidden)
	pending := pendingAttacks(decision.Table)
	risk := BattleRisk{
		TakeNow:         takeNowRisk(decision),
		ContinueDefense: math.Inf(1),
		Transfer:        math.Inf(1),
		CoverProbability: CoverProbability(
			pending,
			decision.Defender,
			visibleHandSize(decision, decision.Defender, len(decision.Hand)),
			decision.TrumpSuit,
			hidden,
		),
	}
	if len(pending) == 0 {
		risk.ContinueDefense = 0
	} else if cost, ok := minimumDefenseCost(decision, hidden); ok {
		risk.ContinueDefense = cost
	}
	if cost, ok := minimumTransferCost(decision, hidden); ok {
		risk.Transfer = cost
	}
	risk.Best = min(risk.TakeNow, risk.ContinueDefense, risk.Transfer)
	return risk
}

// DefenseActionRisk reports the local cost of one defense action.
func DefenseActionRisk(decision *app.DecisionContext, hidden HiddenCards, action domain.Action) float64 {
	if decision == nil || action.Kind != domain.ActionKindDefend {
		return math.Inf(1)
	}
	return defenseSpendCost(action.Card, decision) + rankOpeningPressure(action.Card.Rank, decision, hidden)
}

func takeNowRisk(decision *app.DecisionContext) float64 {
	risk := 1.1
	for _, card := range tableCardsForRisk(decision.Table) {
		risk += 0.60 + cardStickiness(card, decision.TrumpSuit, stockFinality(decision, 0.10))
	}
	return risk
}

func minimumDefenseCost(decision *app.DecisionContext, hidden HiddenCards) (float64, bool) {
	pending := pendingAttacks(decision.Table)
	if len(pending) == 0 {
		return 0, true
	}
	actions := make([]domain.Action, 0, len(decision.LegalActions))
	for _, action := range decision.LegalActions {
		if action.Kind == domain.ActionKindDefend {
			actions = append(actions, action)
		}
	}
	if len(actions) == 0 {
		return 0, false
	}
	used := make([]bool, len(actions))
	best := math.Inf(1)
	var search func(int, float64)
	search = func(index int, cost float64) {
		if cost >= best {
			return
		}
		if index == len(pending) {
			best = cost
			return
		}
		for actionIndex, action := range actions {
			if used[actionIndex] || action.AttackIndex < 0 || action.AttackIndex >= len(decision.Table) {
				continue
			}
			if decision.Table[action.AttackIndex].Attack != pending[index] {
				continue
			}
			used[actionIndex] = true
			search(index+1, cost+DefenseActionRisk(decision, hidden, action))
			used[actionIndex] = false
		}
	}
	search(0, 0)
	if math.IsInf(best, 1) {
		return 0, false
	}
	return best - 0.7, true
}

func minimumTransferCost(decision *app.DecisionContext, hidden HiddenCards) (float64, bool) {
	best := math.Inf(1)
	for _, action := range decision.LegalActions {
		if action.Kind != domain.ActionKindTransfer {
			continue
		}
		cost := defenseSpendCost(action.Card, decision) +
			transferRiskCost(decision, action.Card)*1.6 +
			rankOpeningPressure(action.Card.Rank, decision, hidden)*0.8
		if cost < best {
			best = cost
		}
	}
	if math.IsInf(best, 1) {
		return 0, false
	}
	return best, true
}

func defenseSpendCost(card domain.Card, decision *app.DecisionContext) float64 {
	if !validCard(card) {
		return 2
	}
	finality := stockFinality(decision, 0.10)
	cost := 0.45 + float64(card.Rank-domain.Six)*0.12
	if card.Suit == decision.TrumpSuit {
		cost += 1.25 + (1-finality)*0.75
	}
	return cost
}

func transferRiskCost(decision *app.DecisionContext, card domain.Card) float64 {
	if decision == nil {
		return 1
	}
	finality := stockFinality(decision, 0.10)
	risk := 0.75 + cardStickiness(card, decision.TrumpSuit, finality)*0.65
	for _, tableCard := range tableCardsForRisk(decision.Table) {
		risk += 0.30 + cardStickiness(tableCard, decision.TrumpSuit, finality)*0.25
	}
	if len(decision.HandSizes) == 2 {
		risk += 0.70
	}
	return risk
}

func rankOpeningPressure(rank domain.Rank, decision *app.DecisionContext, hidden HiddenCards) float64 {
	if rank == domain.RankUnknown {
		return 0
	}
	pressure := 0.0
	for _, card := range hidden.UnknownPool {
		if card.Rank == rank {
			pressure += 0.25
		}
	}
	for seat, cards := range hidden.knownHeldGroups() {
		if domain.Seat(seat) == decision.Seat || domain.Seat(seat) == decision.Defender {
			continue
		}
		for _, card := range cards {
			if card.Rank == rank {
				pressure += 0.45
			}
		}
	}
	return pressure
}

func pendingAttacks(table []domain.TablePair) []domain.Card {
	pending := make([]domain.Card, 0, len(table))
	for _, pair := range table {
		if !pair.Defended && validCard(pair.Attack) {
			pending = append(pending, pair.Attack)
		}
	}
	return pending
}
