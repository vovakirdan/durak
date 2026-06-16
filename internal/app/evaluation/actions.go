package evaluation

import (
	"fmt"
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

// RankActions scores legal actions without simulating hidden stock draws.
func RankActions(decision *app.DecisionContext, hidden HiddenCards) []ActionEvaluation {
	if decision == nil {
		return nil
	}
	if len(decision.LegalActions) == 0 {
		return nil
	}
	hidden = ensureHiddenCards(decision, hidden)
	base := Evaluate(decision, hidden)
	results := make([]ActionEvaluation, 0, len(decision.LegalActions))
	for _, action := range decision.LegalActions {
		features := actionFeatures(action, decision, hidden)
		rawDelta := sumFeatures(features)
		score := Clamp(base.Score + rawDelta)
		results = append(results, ActionEvaluation{
			Action:   action,
			Score:    score,
			Delta:    score - base.Score,
			Features: features,
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

func actionFeatures(
	action domain.Action,
	decision *app.DecisionContext,
	hidden HiddenCards,
) []FeatureContribution {
	switch action.Kind {
	case domain.ActionKindDefend:
		return defendActionFeatures(action, decision, hidden)
	case domain.ActionKindTake:
		return takeActionFeatures(decision)
	case domain.ActionKindAttack, domain.ActionKindThrowIn:
		return attackActionFeatures(action, decision, hidden)
	case domain.ActionKindTransfer:
		return transferActionFeatures(action, decision)
	case domain.ActionKindFinishDefense:
		return finishDefenseActionFeatures(decision)
	case domain.ActionKindFinishTake:
		return []FeatureContribution{{
			Name:   FeatureActionKind,
			Score:  20,
			Reason: "finish taking cards",
		}}
	case domain.ActionKindPassThrowIn:
		return passThrowInActionFeatures(decision)
	default:
		return []FeatureContribution{{
			Name:   FeatureActionKind,
			Score:  -40,
			Reason: "unknown action kind",
		}}
	}
}

func defendActionFeatures(
	action domain.Action,
	decision *app.DecisionContext,
	hidden HiddenCards,
) []FeatureContribution {
	score := Score(140)
	reason := "beat an attack card"
	if attack, ok := actionAttackCard(action, decision.Table); ok {
		if cheapest, found := cheapestLegalDefenseAction(action.AttackIndex, decision); found {
			if action.Card == cheapest.Card {
				score += 60
				reason = "cheapest sufficient defense"
			} else {
				score -= Score(cardCost(action.Card, decision.TrumpSuit)-cardCost(cheapest.Card, decision.TrumpSuit)) * 4
				reason = fmt.Sprintf("more expensive than %v", cheapest.Card)
			}
		}
		if action.Card.Suit == decision.TrumpSuit && attack.Suit != decision.TrumpSuit {
			score -= 30
		}
	}
	return []FeatureContribution{
		{Name: FeatureActionKind, Score: 140, Reason: "defend keeps the round alive"},
		{Name: FeatureActionCardEconomy, Score: score - 140, Reason: reason},
		defenseSafetyFeature(action, decision, hidden),
	}
}

func takeActionFeatures(decision *app.DecisionContext) []FeatureContribution {
	score := Score(-220)
	reason := "take table cards"
	if hasLegalDefense(decision) {
		score -= 150
		reason = "taking despite a legal defense"
	}
	score -= Score(len(decision.Table)) * 20
	return []FeatureContribution{{Name: FeatureActionKind, Score: score, Reason: reason}}
}

func attackActionFeatures(
	action domain.Action,
	decision *app.DecisionContext,
	hidden HiddenCards,
) []FeatureContribution {
	kindScore := Score(80)
	if action.Kind == domain.ActionKindThrowIn {
		kindScore = 105
	}
	economy := lowCardEconomy(action.Card, decision.TrumpSuit)
	features := []FeatureContribution{
		{Name: FeatureActionKind, Score: kindScore, Reason: "shed an attacking card"},
		{Name: FeatureActionCardEconomy, Score: economy, Reason: "prefer low non-trumps"},
	}
	if action.Kind == domain.ActionKindThrowIn {
		features = append(features, throwInPressureFeature(action, decision, hidden))
	} else {
		features = append(
			features,
			attackPressureFeature(action, decision, hidden),
			rankMultiplicityFeature(action, decision),
		)
	}
	return features
}

func transferActionFeatures(action domain.Action, decision *app.DecisionContext) []FeatureContribution {
	return []FeatureContribution{
		{Name: FeatureActionKind, Score: 95, Reason: "transfer pressure to another seat"},
		{Name: FeatureActionCardEconomy, Score: lowCardEconomy(action.Card, decision.TrumpSuit), Reason: "prefer cheap transfers"},
	}
}

func passThrowInActionFeatures(decision *app.DecisionContext) []FeatureContribution {
	score := Score(-15)
	reason := "decline optional throw-in"
	if !hasLegalAttackAction(decision) {
		score = 15
		reason = "nothing useful to throw in"
	}
	return []FeatureContribution{{Name: FeatureActionKind, Score: score, Reason: reason}}
}

func finishDefenseActionFeatures(decision *app.DecisionContext) []FeatureContribution {
	score := Score(80)
	reason := "finish a successful defense"
	if hasLegalThrowInAction(decision) {
		score -= 35
		reason = "end pressure while throw-in is available"
		if decision.StockCount <= 3 {
			score -= 25
			reason = "end late pressure while throw-in is available"
		}
	}
	return []FeatureContribution{{Name: FeatureActionKind, Score: score, Reason: reason}}
}

func throwInPressureFeature(
	action domain.Action,
	decision *app.DecisionContext,
	hidden HiddenCards,
) FeatureContribution {
	score := Score(85)
	reason := "continue throw-in pressure"
	if decision.Phase == domain.MatchPhaseTaking {
		score = 190
		reason = "add cards to defender taking"
	}
	if decision.StockCount == 0 {
		score += 55
		reason += " with empty stock"
	} else if decision.StockCount <= 3 {
		score += 30
		reason += " in late stock"
	}
	score += Score(len(decision.Table)) * 12
	if rankCount(decision.Hand, action.Card.Rank) > 1 {
		score += 25
		reason += " using rank group"
	}
	if knownDefenderCanBeat(action.Card, decision, hidden) {
		score -= 70
		reason += " but known defender card can cover"
	}
	if action.Card.Suit == decision.TrumpSuit && decision.StockCount > 3 {
		score -= 60
		reason += " while spending trump early"
	}
	return FeatureContribution{Name: FeatureActionPressure, Score: score, Reason: reason}
}

func attackPressureFeature(
	action domain.Action,
	decision *app.DecisionContext,
	hidden HiddenCards,
) FeatureContribution {
	if !validCard(action.Card) {
		return FeatureContribution{Name: FeatureActionPressure}
	}
	score := Score(0)
	reason := "attack pressure"
	if knownDefenderCanBeat(action.Card, decision, hidden) {
		score -= 35
		reason = "known defender card can cover"
	} else if knownDefenderHandSize(decision.Defender, hidden) > 0 {
		score += 55
		reason = "attack known weak defender cards"
	}
	return FeatureContribution{Name: FeatureActionPressure, Score: score, Reason: reason}
}

func rankMultiplicityFeature(action domain.Action, decision *app.DecisionContext) FeatureContribution {
	count := rankCount(decision.Hand, action.Card.Rank)
	if count <= 1 {
		return FeatureContribution{Name: FeatureActionPressure}
	}
	weight := Score(20)
	reason := "attack rank keeps same-rank follow-up"
	return FeatureContribution{
		Name:   FeatureActionPressure,
		Score:  Score(count-1) * weight,
		Reason: reason,
	}
}

func knownDefenderHandSize(defender domain.Seat, hidden HiddenCards) int {
	groups := hidden.knownHeldGroups()
	if defender == domain.NoSeat || int(defender) < 0 || int(defender) >= len(groups) {
		return 0
	}
	return len(groups[int(defender)])
}

func defenseSafetyFeature(
	action domain.Action,
	decision *app.DecisionContext,
	hidden HiddenCards,
) FeatureContribution {
	tableRanks := tableRanksAfterDefense(action, decision)
	knownThreats := 0
	for seat, cards := range hidden.knownHeldGroups() {
		if domain.Seat(seat) == decision.Seat {
			continue
		}
		for _, card := range cards {
			if tableRanks[card.Rank] {
				knownThreats++
			}
		}
	}
	if knownThreats == 0 {
		return FeatureContribution{Name: FeatureActionDefenseSafety}
	}
	return FeatureContribution{
		Name:   FeatureActionDefenseSafety,
		Score:  -Score(knownThreats) * 45,
		Reason: fmt.Sprintf("%d known opponent throw-in cards after defense", knownThreats),
	}
}

func tableRanksAfterDefense(action domain.Action, decision *app.DecisionContext) map[domain.Rank]bool {
	ranks := make(map[domain.Rank]bool, len(decision.Table)*2+1)
	for index, pair := range decision.Table {
		if validCard(pair.Attack) {
			ranks[pair.Attack.Rank] = true
		}
		if pair.Defended && validCard(pair.Defense) {
			ranks[pair.Defense.Rank] = true
		}
		if index == action.AttackIndex && validCard(action.Card) {
			ranks[action.Card.Rank] = true
		}
	}
	return ranks
}

func knownDefenderCanBeat(card domain.Card, decision *app.DecisionContext, hidden HiddenCards) bool {
	defender := decision.Defender
	groups := hidden.knownHeldGroups()
	if defender == domain.NoSeat || int(defender) < 0 || int(defender) >= len(groups) {
		return false
	}
	for _, candidate := range groups[int(defender)] {
		if domain.CanBeat(card, candidate, decision.TrumpSuit) {
			return true
		}
	}
	return false
}

func lowCardEconomy(card domain.Card, trumpSuit domain.Suit) Score {
	if !validCard(card) {
		return 0
	}
	score := Score(domain.Ace-card.Rank) * 6
	if card.Suit == trumpSuit {
		score -= 90
	}
	if card.Rank >= domain.Queen {
		score -= 25
	}
	return score
}

func hasLegalDefense(decision *app.DecisionContext) bool {
	for _, action := range decision.LegalActions {
		if action.Kind == domain.ActionKindDefend {
			return true
		}
	}
	return false
}

func hasLegalAttackAction(decision *app.DecisionContext) bool {
	for _, action := range decision.LegalActions {
		switch action.Kind {
		case domain.ActionKindAttack, domain.ActionKindThrowIn, domain.ActionKindTransfer:
			return true
		}
	}
	return false
}

func hasLegalThrowInAction(decision *app.DecisionContext) bool {
	for _, action := range decision.LegalActions {
		if action.Kind == domain.ActionKindThrowIn {
			return true
		}
	}
	return false
}

func rankCount(cards []domain.Card, rank domain.Rank) int {
	count := 0
	for _, card := range cards {
		if card.Rank == rank {
			count++
		}
	}
	return count
}

func cheapestLegalDefenseAction(
	attackIndex int,
	decision *app.DecisionContext,
) (domain.Action, bool) {
	var best domain.Action
	found := false
	for _, candidate := range decision.LegalActions {
		if candidate.Kind != domain.ActionKindDefend || candidate.AttackIndex != attackIndex {
			continue
		}
		if !found || cardCost(candidate.Card, decision.TrumpSuit) < cardCost(best.Card, decision.TrumpSuit) {
			best = candidate
			found = true
		}
	}
	return best, found
}

func actionAttackCard(action domain.Action, table []domain.TablePair) (domain.Card, bool) {
	if action.AttackIndex < 0 || action.AttackIndex >= len(table) {
		return domain.Card{}, false
	}
	return table[action.AttackIndex].Attack, true
}

func sumFeatures(features []FeatureContribution) Score {
	var score Score
	for _, feature := range features {
		score += feature.Score
	}
	return score
}
