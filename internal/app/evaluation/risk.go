package evaluation

import (
	"fmt"
	"math"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	// FeatureRiskScore names the final score contribution emitted by RiskModel.
	FeatureRiskScore = "risk_score"
	// FeatureRiskHandBurden names the evaluated seat's effective hand burden.
	FeatureRiskHandBurden = "risk_hand_burden"
	// FeatureRiskOutlet names near-term opportunities to shed cards.
	FeatureRiskOutlet = "risk_outlet"
	// FeatureRiskDefense names cheap-defense stability.
	FeatureRiskDefense = "risk_defense_stability"
	// FeatureRiskInitiative names current role and tempo value.
	FeatureRiskInitiative = "risk_initiative"
	// FeatureRiskBattleThreat names current defender battle pressure.
	FeatureRiskBattleThreat = "risk_battle_threat"
)

// RiskWeights are Excel-tunable coefficients measured in bad-card units.
type RiskWeights struct {
	Beta             float64
	HandBurden       float64
	BattleRisk       float64
	Outlet           float64
	DefenseStability float64
	Initiative       float64
}

// RiskModel scores seats by estimating who is most likely to remain durak.
type RiskModel struct {
	Weights RiskWeights
}

// RiskComponents exposes the model terms for traces and parity tests.
type RiskComponents struct {
	Seat             domain.Seat
	HandBurden       float64
	BattleRisk       float64
	Outlet           float64
	DefenseStability float64
	Initiative       float64
	RiskIndex        float64
	DurakProbability float64
	Score            Score
}

// DefaultRiskModel returns the spreadsheet-aligned v0.1 model.
func DefaultRiskModel() RiskModel {
	return RiskModel{Weights: RiskWeights{
		Beta:             0.30,
		HandBurden:       1.00,
		BattleRisk:       1.20,
		Outlet:           0.80,
		DefenseStability: 0.90,
		Initiative:       0.50,
	}}
}

// Evaluate scores the evaluated seat from fair public information.
func (m RiskModel) Evaluate(decision *app.DecisionContext, hidden HiddenCards) PositionEvaluation {
	if decision == nil {
		return NewPositionEvaluation(domain.NoSeat, 0)
	}
	if m.Weights == (RiskWeights{}) {
		m = DefaultRiskModel()
	}
	hidden = ensureHiddenCards(decision, hidden)
	if decision.Phase == domain.MatchPhaseComplete {
		return terminalRiskEvaluation(decision)
	}
	components := m.Components(decision, hidden)
	seat := int(decision.Seat)
	if seat < 0 || seat >= len(components) {
		return NewPositionEvaluation(decision.Seat, hiddenConfidence(hidden))
	}
	self := components[seat]
	return PositionEvaluation{
		Seat:       decision.Seat,
		Score:      self.Score,
		Confidence: hiddenConfidence(hidden),
		Features: []FeatureContribution{
			{Name: FeatureRiskScore, Score: self.Score, Reason: fmt.Sprintf("durak probability %.3f", self.DurakProbability)},
			{Name: FeatureRiskHandBurden, Score: riskTermScore(-self.HandBurden), Reason: "effective hand burden"},
			{Name: FeatureRiskOutlet, Score: riskTermScore(self.Outlet), Reason: "near-term card outlet"},
			{Name: FeatureRiskDefense, Score: riskTermScore(self.DefenseStability), Reason: "defense stability"},
			{Name: FeatureRiskInitiative, Score: riskTermScore(self.Initiative), Reason: "role and initiative"},
			{Name: FeatureRiskBattleThreat, Score: riskTermScore(-self.BattleRisk), Reason: "current battle risk"},
		},
	}
}

// Components returns every seat's model terms under the current belief.
func (m RiskModel) Components(decision *app.DecisionContext, hidden HiddenCards) []RiskComponents {
	if decision == nil {
		return nil
	}
	if m.Weights == (RiskWeights{}) {
		m = DefaultRiskModel()
	}
	hidden = ensureHiddenCards(decision, hidden)
	components := make([]RiskComponents, len(decision.HandSizes))
	risks := make([]float64, len(decision.HandSizes))
	for seat := range components {
		seatID := domain.Seat(seat)
		terms := m.seatTerms(decision, hidden, seatID)
		risk := m.Weights.HandBurden*terms.HandBurden +
			m.Weights.BattleRisk*terms.BattleRisk -
			m.Weights.Outlet*terms.Outlet -
			m.Weights.DefenseStability*terms.DefenseStability -
			m.Weights.Initiative*terms.Initiative
		terms.Seat = seatID
		terms.RiskIndex = risk
		components[seat] = terms
		risks[seat] = risk
	}
	active := activeRiskSeats(decision)
	for seat := range components {
		probability := softmaxProbability(risks, seat, m.Weights.Beta)
		components[seat].DurakProbability = probability
		components[seat].Score = ScoreFromDurakProbability(probability, active)
	}
	return components
}

func (m RiskModel) seatTerms(decision *app.DecisionContext, hidden HiddenCards, seat domain.Seat) RiskComponents {
	hand := knownSeatHand(decision, hidden, seat)
	size := visibleHandSize(decision, seat, len(hand))
	return RiskComponents{
		HandBurden:       m.handBurden(decision, hidden, seat, hand, size),
		BattleRisk:       m.currentBattleRisk(decision, hidden, seat),
		Outlet:           m.outletPotential(decision, hidden, seat, hand, size),
		DefenseStability: m.defenseStability(decision, hidden, hand, size),
		Initiative:       m.initiativeValue(decision, seat),
	}
}

func (m RiskModel) handBurden(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
	hand []domain.Card,
	size int,
) float64 {
	sticky := 0.0
	for _, card := range hand {
		sticky += cardStickinessForSeat(card, decision, seat, hand)
	}
	unknownSlots := max(0, size-len(hand))
	if unknownSlots > 0 {
		sticky += float64(unknownSlots) * expectedStickiness(decision, hidden, seat, hand)
	}
	return phaseWeight(decision) * sticky
}

func cardStickinessForSeat(
	card domain.Card,
	decision *app.DecisionContext,
	seat domain.Seat,
	hand []domain.Card,
) float64 {
	if !validCard(card) {
		return 1
	}
	e := endgameFactor(decision)
	if card.Suit == decision.TrumpSuit {
		return 1 - (0.15*(1-e) + 0.5*e)
	}
	out := float64(8-rankValue(card)) / 8
	if seat == decision.Attacker {
		out *= 1.2
	} else {
		out *= 0.6
	}
	if rankCount(hand, card.Rank) >= 2 {
		out += 0.25
	}
	if rankOnProjectedTable(decision.Table, card.Rank) {
		out += 0.30
	}
	return 1 - math.Min(1, out)
}

func expectedStickiness(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
	known []domain.Card,
) float64 {
	if len(hidden.UnknownPool) == 0 {
		return 1
	}
	total := 0.0
	for _, card := range hidden.UnknownPool {
		total += cardStickinessForSeat(card, decision, seat, append(known, card))
	}
	return total / float64(len(hidden.UnknownPool))
}

func (m RiskModel) currentBattleRisk(decision *app.DecisionContext, hidden HiddenCards, seat domain.Seat) float64 {
	if seat == decision.Defender &&
		(decision.Phase == domain.MatchPhaseDefense || decision.Phase == domain.MatchPhaseTaking) {
		risk := EvaluateBattleRiskForSeat(decision, hidden, seat)
		return risk.Best + risk.TablePressure*0.3
	}
	if seat != decision.Attacker && seat != decision.Defender && activeProjectedSeatLike(decision, seat) {
		return 0.5
	}
	return 0
}

func (m RiskModel) outletPotential(
	decision *app.DecisionContext,
	hidden HiddenCards,
	seat domain.Seat,
	hand []domain.Card,
	size int,
) float64 {
	outlet := 0.0
	if seat == decision.Attacker {
		outlet += expectedCardCount(hidden, hand, size, func(card domain.Card) bool {
			return card.Suit != decision.TrumpSuit && rankValue(card) < 5
		})
	}
	outlet += float64(rankPairs(hand)) * 0.5
	return outlet
}

func (m RiskModel) defenseStability(
	decision *app.DecisionContext,
	hidden HiddenCards,
	hand []domain.Card,
	size int,
) float64 {
	if size <= 0 {
		return 0
	}
	trumps := expectedCardCount(hidden, hand, size, func(card domain.Card) bool {
		return card.Suit == decision.TrumpSuit
	})
	highNonTrumps := expectedCardCount(hidden, hand, size, func(card domain.Card) bool {
		return card.Suit != decision.TrumpSuit && rankValue(card) > 4
	})
	return (trumps*1.5 + highNonTrumps*0.5) / float64(size)
}

func (m RiskModel) initiativeValue(decision *app.DecisionContext, seat domain.Seat) float64 {
	switch seat {
	case decision.Attacker:
		return defaultInitiativeBonus
	case decision.Defender:
		return -0.5
	default:
		return 0
	}
}

func expectedCardCount(
	hidden HiddenCards,
	known []domain.Card,
	size int,
	match func(domain.Card) bool,
) float64 {
	total := 0.0
	for _, card := range known {
		if match(card) {
			total++
		}
	}
	unknownSlots := max(0, size-len(known))
	if unknownSlots == 0 || len(hidden.UnknownPool) == 0 {
		return total
	}
	matches := 0
	for _, card := range hidden.UnknownPool {
		if match(card) {
			matches++
		}
	}
	return total + float64(unknownSlots*matches)/float64(len(hidden.UnknownPool))
}

func phaseWeight(decision *app.DecisionContext) float64 {
	return 0.35 + 0.65*endgameFactor(decision)
}

func endgameFactor(decision *app.DecisionContext) float64 {
	if decision == nil {
		return 1
	}
	hands := 0
	for _, size := range decision.HandSizes {
		hands += size
	}
	total := hands + decision.StockCount
	if total <= 0 {
		return 1
	}
	return float64(hands) / float64(total)
}

func activeRiskSeats(decision *app.DecisionContext) int {
	active := 0
	for _, size := range decision.HandSizes {
		if size > 0 || decision.StockCount > 0 || len(decision.Table) > 0 {
			active++
		}
	}
	if active == 0 {
		active = len(decision.HandSizes)
	}
	if active < 1 {
		return 1
	}
	return active
}

func activeProjectedSeatLike(decision *app.DecisionContext, seat domain.Seat) bool {
	return seat != domain.NoSeat && int(seat) >= 0 && int(seat) < len(decision.HandSizes) &&
		(decision.HandSizes[int(seat)] > 0 || decision.StockCount > 0 || len(decision.Table) > 0)
}

func softmaxProbability(risks []float64, seat int, beta float64) float64 {
	if len(risks) == 0 || seat < 0 || seat >= len(risks) {
		return 1
	}
	maxRisk := risks[0] * beta
	for _, risk := range risks[1:] {
		if scaled := risk * beta; scaled > maxRisk {
			maxRisk = scaled
		}
	}
	total := 0.0
	selected := 0.0
	for index, risk := range risks {
		value := math.Exp(risk*beta - maxRisk)
		total += value
		if index == seat {
			selected = value
		}
	}
	if total == 0 {
		return 1 / float64(len(risks))
	}
	return selected / total
}

func terminalRiskEvaluation(decision *app.DecisionContext) PositionEvaluation {
	switch {
	case decision.Winner == decision.Seat:
		return NewPositionEvaluation(decision.Seat, 100, FeatureContribution{Name: FeatureTerminal, Score: MaxScore})
	case decision.Loser == decision.Seat:
		return NewPositionEvaluation(decision.Seat, 100, FeatureContribution{Name: FeatureTerminal, Score: MinScore})
	default:
		return NewPositionEvaluation(decision.Seat, 100)
	}
}

func riskTermScore(value float64) Score {
	return Clamp(Score(math.Round(value * 100)))
}

func knownSeatHand(decision *app.DecisionContext, hidden HiddenCards, seat domain.Seat) []domain.Card {
	if seat == decision.Seat {
		return decision.Hand
	}
	groups := hidden.knownHeldGroups()
	if seat == domain.NoSeat || int(seat) < 0 || int(seat) >= len(groups) {
		return nil
	}
	return groups[int(seat)]
}

func visibleHandSize(decision *app.DecisionContext, seat domain.Seat, fallback int) int {
	if seat != domain.NoSeat && int(seat) >= 0 && int(seat) < len(decision.HandSizes) {
		return decision.HandSizes[int(seat)]
	}
	return fallback
}

func rankValue(card domain.Card) int {
	if !validCard(card) {
		return 0
	}
	return int(card.Rank - domain.Six)
}

func rankOnProjectedTable(table []domain.TablePair, rank domain.Rank) bool {
	for _, pair := range table {
		if pair.Attack.Rank == rank || pair.Defended && pair.Defense.Rank == rank {
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

func rankPairs(cards []domain.Card) int {
	counts := make(map[domain.Rank]int)
	pairs := 0
	for _, card := range cards {
		counts[card.Rank]++
		if counts[card.Rank] == 2 {
			pairs++
		}
	}
	return pairs
}

func tableCardsForRisk(table []domain.TablePair) []domain.Card {
	cards := make([]domain.Card, 0, len(table)*2)
	for _, pair := range table {
		if validCard(pair.Attack) {
			cards = append(cards, pair.Attack)
		}
		if pair.Defended && validCard(pair.Defense) {
			cards = append(cards, pair.Defense)
		}
	}
	return cards
}
