package evaluation

import (
	"math"
	"slices"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	defaultTrumpPremium    = 2.5
	defaultRankOpeningBase = 0.5
	defaultSkipPenalty     = 2.0
	defaultInitiativeBonus = 1.5
)

// BattleBranch identifies the expected defender response selected by risk.
type BattleBranch string

const (
	// BattleBranchDefend means the defender is expected to cover the table.
	BattleBranchDefend BattleBranch = "defend"
	// BattleBranchTake means the defender is expected to take the table.
	BattleBranchTake BattleBranch = "take"
	// BattleBranchTransfer means the defender is expected to transfer.
	BattleBranchTransfer BattleBranch = "transfer"

	battleBranchNone BattleBranch = ""
)

// BattleRisk compares defender options in bad-card units.
type BattleRisk struct {
	TakeNow          float64
	ContinueDefense  float64
	Transfer         float64
	Best             float64
	CoverProbability float64
	TablePressure    float64
	BestBranch       BattleBranch
	BestAction       domain.Action
}

// EvaluateBattleRisk evaluates the current defender's local battle problem.
func EvaluateBattleRisk(decision *app.DecisionContext, hidden HiddenCards) BattleRisk {
	if decision == nil {
		return BattleRisk{}
	}
	return EvaluateBattleRiskForSeat(decision, hidden, decision.Defender)
}

// EvaluateBattleRiskForSeat estimates a defender's best branch from seat-view belief.
func EvaluateBattleRiskForSeat(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
) BattleRisk {
	if decision == nil || seat != decision.Defender {
		return BattleRisk{}
	}
	if decision.Phase != domain.MatchPhaseDefense && decision.Phase != domain.MatchPhaseTaking {
		return BattleRisk{}
	}
	hidden = ensureHiddenCards(decision, hidden)
	pending := pendingAttacks(decision.Table)
	risk := BattleRisk{
		TakeNow:          takeNowRisk(decision),
		ContinueDefense:  math.Inf(1),
		Transfer:         math.Inf(1),
		CoverProbability: CoverProbability(pending, seat, visibleHandSize(decision, seat, 0), decision.TrumpSuit, hidden),
		TablePressure:    tablePressure(decision, hidden),
		BestBranch:       BattleBranchTake,
	}
	if len(pending) == 0 {
		risk.ContinueDefense = 0
		risk.BestBranch = BattleBranchDefend
	} else if cost, action, ok := minimumDefenseCostForSeat(decision, hidden, seat); ok {
		risk.ContinueDefense = cost
		risk.BestAction = action
	}
	if cost, action, ok := minimumTransferCostForSeat(decision, hidden, seat); ok {
		risk.Transfer = cost
		if risk.BestAction == (domain.Action{}) {
			risk.BestAction = action
		}
	}
	risk.Best, risk.BestBranch = bestBattleBranch(&risk)
	if risk.BestBranch == BattleBranchTransfer {
		if _, action, ok := minimumTransferCostForSeat(decision, hidden, seat); ok {
			risk.BestAction = action
		}
	}
	return risk
}

// DefenseActionRisk reports the local cost of one defense action.
func DefenseActionRisk(decision *app.DecisionContext, hidden HiddenCards, action domain.Action) float64 {
	if decision == nil || action.Kind != domain.ActionKindDefend {
		return math.Inf(1)
	}
	return defenseSpendCost(action.Card, decision) + rankOpeningPressure(action.Card.Rank, decision, hidden)
}

func bestBattleBranch(risk *BattleRisk) (float64, BattleBranch) {
	best := risk.TakeNow
	branch := BattleBranchTake
	if risk.ContinueDefense <= best {
		best = risk.ContinueDefense
		branch = BattleBranchDefend
	}
	if risk.Transfer < best {
		best = risk.Transfer
		branch = BattleBranchTransfer
	}
	return best, branch
}

func takeNowRisk(decision *app.DecisionContext) float64 {
	risk := defaultSkipPenalty
	for _, card := range tableCardsForRisk(decision.Table) {
		risk += baseCardCost(card)
	}
	return risk
}

func minimumDefenseCostForSeat(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
) (float64, domain.Action, bool) {
	if seat == decision.Seat {
		if cost, action, ok := minimumLegalDefenseCost(decision, hidden); ok {
			return cost, action, true
		}
	}
	hand := knownSeatHand(decision, hidden, seat)
	if cost, action, ok := minimumKnownDefenseCost(decision, hidden, seat, hand); ok {
		return cost, action, true
	}
	return minimumExpectedDefenseCost(decision, hidden, seat)
}

func minimumLegalDefenseCost(
	decision *app.DecisionContext,
	hidden HiddenCards,
) (float64, domain.Action, bool) {
	pending := pendingAttacks(decision.Table)
	if len(pending) == 0 {
		return 0, domain.Action{}, true
	}
	actions := make([]domain.Action, 0, len(decision.LegalActions))
	for _, action := range decision.LegalActions {
		if action.Kind == domain.ActionKindDefend {
			actions = append(actions, action)
		}
	}
	if len(actions) == 0 {
		return 0, domain.Action{}, false
	}
	used := make([]bool, len(actions))
	best := math.Inf(1)
	var bestFirst domain.Action
	var search func(int, float64, domain.Action)
	search = func(index int, cost float64, first domain.Action) {
		if cost >= best {
			return
		}
		if index == len(pending) {
			best = cost
			bestFirst = first
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
			nextFirst := first
			if nextFirst == (domain.Action{}) {
				nextFirst = action
			}
			search(index+1, cost+DefenseActionRisk(decision, hidden, action), nextFirst)
			used[actionIndex] = false
		}
	}
	search(0, 0, domain.Action{})
	if math.IsInf(best, 1) {
		return 0, domain.Action{}, false
	}
	return best, bestFirst, true
}

func minimumKnownDefenseCost(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
	hand []domain.Card,
) (float64, domain.Action, bool) {
	pending := pendingAttacks(decision.Table)
	if len(pending) == 0 {
		return 0, domain.Action{}, true
	}
	if len(hand) < len(pending) || !CanCoverAll(pending, hand, decision.TrumpSuit) {
		return 0, domain.Action{}, false
	}
	used := make([]bool, len(hand))
	best := math.Inf(1)
	var bestFirst domain.Action
	var search func(int, float64, domain.Action)
	search = func(index int, cost float64, first domain.Action) {
		if cost >= best {
			return
		}
		if index == len(pending) {
			best = cost
			bestFirst = first
			return
		}
		for handIndex, card := range hand {
			if used[handIndex] || !domain.CanBeat(pending[index], card, decision.TrumpSuit) {
				continue
			}
			used[handIndex] = true
			action := domain.Action{
				Kind:        domain.ActionKindDefend,
				Seat:        seat,
				Card:        card,
				AttackIndex: pendingAttackIndex(decision.Table, pending[index]),
			}
			nextFirst := first
			if nextFirst == (domain.Action{}) {
				nextFirst = action
			}
			search(index+1, cost+DefenseActionRisk(decision, hidden, action), nextFirst)
			used[handIndex] = false
		}
	}
	search(0, 0, domain.Action{})
	if math.IsInf(best, 1) {
		return 0, domain.Action{}, false
	}
	return best, bestFirst, true
}

func minimumExpectedDefenseCost(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
) (float64, domain.Action, bool) {
	pending := pendingAttacks(decision.Table)
	if len(pending) == 0 {
		return 0, domain.Action{}, true
	}
	pCover := CoverProbability(pending, seat, visibleHandSize(decision, seat, 0), decision.TrumpSuit, hidden)
	if pCover <= 0 {
		return 0, domain.Action{}, false
	}
	cost := 0.0
	var first domain.Action
	for index, attack := range pending {
		card, ok := expectedCheapestBeater(attack, hidden.UnknownPool, decision)
		if !ok {
			continue
		}
		action := domain.Action{Kind: domain.ActionKindDefend, Seat: seat, Card: card, AttackIndex: pendingAttackIndex(decision.Table, attack)}
		if index == 0 {
			first = action
		}
		cost += DefenseActionRisk(decision, hidden, action)
	}
	if cost == 0 {
		cost = float64(len(pending)) * (1.0 + tablePressure(decision, hidden)*0.15)
	}
	return cost / math.Max(pCover, 0.25), first, true
}

func expectedCheapestBeater(
	attack domain.Card,
	unknown []domain.Card,
	decision *app.DecisionContext,
) (domain.Card, bool) {
	var best domain.Card
	found := false
	for _, card := range unknown {
		if !domain.CanBeat(attack, card, decision.TrumpSuit) {
			continue
		}
		if !found || defenseSpendCost(card, decision) < defenseSpendCost(best, decision) {
			best = card
			found = true
		}
	}
	return best, found
}

func minimumTransferCostForSeat(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
) (float64, domain.Action, bool) {
	if seat == decision.Seat {
		best := math.Inf(1)
		var bestAction domain.Action
		for _, action := range decision.LegalActions {
			if action.Kind != domain.ActionKindTransfer {
				continue
			}
			cost := transferCost(action.Card, decision)
			if cost < best {
				best = cost
				bestAction = action
			}
		}
		if !math.IsInf(best, 1) {
			return best, bestAction, true
		}
	}
	if hasDefendedProjectedTableCards(decision.Table) {
		return 0, domain.Action{}, false
	}
	rank, ok := transferRank(decision.Table)
	if !ok {
		return 0, domain.Action{}, false
	}
	hand := knownSeatHand(decision, hidden, seat)
	if card, ok := cheapestRankCard(hand, rank, decision); ok {
		action := domain.Action{Kind: domain.ActionKindTransfer, Seat: seat, Card: card}
		return transferCost(card, decision), action, true
	}
	card, probability := expectedRankCard(rank, hidden.UnknownPool, visibleHandSize(decision, seat, 0)-len(hand), decision)
	if probability <= 0 {
		return 0, domain.Action{}, false
	}
	action := domain.Action{Kind: domain.ActionKindTransfer, Seat: seat, Card: card}
	cost := transferCost(card, decision) / math.Max(probability, 0.25)
	return cost, action, true
}

func transferCost(card domain.Card, decision *app.DecisionContext) float64 {
	return math.Max(0, transferCardCost(card, decision)-defaultInitiativeBonus*0.5)
}

func defenseSpendCost(card domain.Card, decision *app.DecisionContext) float64 {
	if !validCard(card) {
		return 2
	}
	cost := baseCardCost(card)
	if card.Suit == decision.TrumpSuit {
		cost += defaultTrumpPremium * (1 - endgameFactor(decision)*0.5)
	}
	return cost
}

func transferCardCost(card domain.Card, decision *app.DecisionContext) float64 {
	cost := baseCardCost(card)
	if card.Suit == decision.TrumpSuit {
		cost += defaultTrumpPremium
	}
	return cost
}

func baseCardCost(card domain.Card) float64 {
	if !validCard(card) {
		return 1
	}
	return float64(rankValue(card)+1) / 9
}

func rankOpeningPressure(rank domain.Rank, decision *app.DecisionContext, hidden HiddenCards) float64 {
	if rank == domain.RankUnknown {
		return 0
	}
	return defaultRankOpeningBase * freeRankCount(rank, decision, hidden)
}

func tablePressure(decision *app.DecisionContext, hidden HiddenCards) float64 {
	pressure := 0.0
	for _, rank := range openTableRanks(decision.Table) {
		pressure += freeRankCount(rank, decision, hidden)
	}
	return pressure
}

func freeRankCount(rank domain.Rank, decision *app.DecisionContext, hidden HiddenCards) float64 {
	if decision == nil || rank == domain.RankUnknown {
		return 0
	}
	total := 0
	for _, card := range domain.NewDeck36() {
		if card.Rank == rank {
			total++
		}
	}
	for _, pair := range decision.Table {
		if pair.Attack.Rank == rank {
			total--
		}
		if pair.Defended && pair.Defense.Rank == rank {
			total--
		}
	}
	for _, card := range knownSeatHand(decision, hidden, decision.Defender) {
		if card.Rank == rank {
			total--
		}
	}
	for _, card := range decision.PublicMemory.Discard {
		if card.Rank == rank {
			total--
		}
	}
	if total < 0 {
		return 0
	}
	return float64(total)
}

func openTableRanks(table []domain.TablePair) []domain.Rank {
	ranks := make([]domain.Rank, 0, len(table)*2)
	for _, pair := range table {
		if pair.Attack.Rank != domain.RankUnknown && !slices.Contains(ranks, pair.Attack.Rank) {
			ranks = append(ranks, pair.Attack.Rank)
		}
		if pair.Defended && pair.Defense.Rank != domain.RankUnknown && !slices.Contains(ranks, pair.Defense.Rank) {
			ranks = append(ranks, pair.Defense.Rank)
		}
	}
	return ranks
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

func pendingAttackIndex(table []domain.TablePair, attack domain.Card) int {
	for index, pair := range table {
		if !pair.Defended && pair.Attack == attack {
			return index
		}
	}
	return -1
}

func transferRank(table []domain.TablePair) (domain.Rank, bool) {
	for _, pair := range table {
		if !pair.Defended && pair.Attack.Rank != domain.RankUnknown {
			return pair.Attack.Rank, true
		}
	}
	return domain.RankUnknown, false
}

func cheapestRankCard(cards []domain.Card, rank domain.Rank, decision *app.DecisionContext) (domain.Card, bool) {
	var best domain.Card
	found := false
	for _, card := range cards {
		if card.Rank != rank {
			continue
		}
		if !found || transferCardCost(card, decision) < transferCardCost(best, decision) {
			best = card
			found = true
		}
	}
	return best, found
}

func expectedRankCard(
	rank domain.Rank,
	unknown []domain.Card,
	unknownSlots int,
	decision *app.DecisionContext,
) (best domain.Card, probability float64) {
	if unknownSlots <= 0 || len(unknown) == 0 {
		return domain.Card{}, 0
	}
	matches := 0
	for _, card := range unknown {
		if card.Rank != rank {
			continue
		}
		matches++
		if best == (domain.Card{}) || transferCardCost(card, decision) < transferCardCost(best, decision) {
			best = card
		}
	}
	return best, hypergeometricAtLeastOne(len(unknown), matches, unknownSlots)
}

func hasDefendedProjectedTableCards(table []domain.TablePair) bool {
	for _, pair := range table {
		if pair.Defended {
			return true
		}
	}
	return false
}
