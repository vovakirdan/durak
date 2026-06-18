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
	Beta              float64
	StockFinalityBase float64
	HandBurden        float64
	Threat            float64
	Outlet            float64
	DefenseStability  float64
	Initiative        float64
}

// RiskModel scores seats by estimating who is most likely to remain durak.
type RiskModel struct {
	Weights RiskWeights
}

// DefaultRiskModel returns conservative first-pass weights.
func DefaultRiskModel() RiskModel {
	return RiskModel{Weights: RiskWeights{
		Beta:              0.55,
		StockFinalityBase: 0.10,
		HandBurden:        1.00,
		Threat:            1.00,
		Outlet:            0.35,
		DefenseStability:  0.30,
		Initiative:        0.45,
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
	risks := m.riskIndices(decision, hidden)
	seat := int(decision.Seat)
	if seat < 0 || seat >= len(risks) {
		return NewPositionEvaluation(decision.Seat, hiddenConfidence(hidden))
	}
	probability := softmaxProbability(risks, seat, m.Weights.Beta)
	score := ScoreFromDurakProbability(probability, activeRiskSeats(decision))
	self := m.seatTerms(decision, hidden, decision.Seat)
	return PositionEvaluation{
		Seat:       decision.Seat,
		Score:      score,
		Confidence: hiddenConfidence(hidden),
		Features: []FeatureContribution{
			{Name: FeatureRiskScore, Score: score, Reason: fmt.Sprintf("durak probability %.3f", probability)},
			{Name: FeatureRiskHandBurden, Score: riskTermScore(-self.handBurden), Reason: "effective hand burden"},
			{Name: FeatureRiskOutlet, Score: riskTermScore(self.outlet), Reason: "near-term card outlet"},
			{Name: FeatureRiskDefense, Score: riskTermScore(self.defenseStability), Reason: "cheap defense stability"},
			{Name: FeatureRiskInitiative, Score: riskTermScore(self.initiative), Reason: "role and initiative"},
			{Name: FeatureRiskBattleThreat, Score: riskTermScore(-self.battleThreat), Reason: "current battle threat"},
		},
	}
}

type riskTerms struct {
	handBurden       float64
	battleThreat     float64
	outlet           float64
	defenseStability float64
	initiative       float64
}

func (m RiskModel) riskIndices(decision *app.DecisionContext, hidden HiddenCards) []float64 {
	risks := make([]float64, len(decision.HandSizes))
	for seat := range risks {
		terms := m.seatTerms(decision, hidden, domain.Seat(seat))
		risks[seat] = m.Weights.HandBurden*terms.handBurden +
			m.Weights.Threat*terms.battleThreat -
			m.Weights.Outlet*terms.outlet -
			m.Weights.DefenseStability*terms.defenseStability -
			m.Weights.Initiative*terms.initiative
	}
	return risks
}

func (m RiskModel) seatTerms(decision *app.DecisionContext, hidden HiddenCards, seat domain.Seat) riskTerms {
	hand := knownSeatHand(decision, hidden, seat)
	size := visibleHandSize(decision, seat, len(hand))
	return riskTerms{
		handBurden:       m.handBurden(decision, hand, size),
		battleThreat:     currentBattleThreat(decision, hidden, seat),
		outlet:           outletPotential(decision, hand, seat),
		defenseStability: defenseStability(decision, hand),
		initiative:       initiativeValue(decision, seat),
	}
}

func (m RiskModel) handBurden(decision *app.DecisionContext, hand []domain.Card, size int) float64 {
	finality := stockFinality(decision, m.Weights.StockFinalityBase)
	if len(hand) == 0 {
		overload := max(0, size-6)
		return (1-finality)*float64(overload) + finality*float64(size)
	}
	var sticky float64
	for _, card := range hand {
		sticky += cardStickiness(card, decision.TrumpSuit, finality)
	}
	overload := max(0, size-6)
	return (1-finality)*float64(overload) + finality*sticky
}

func stockFinality(decision *app.DecisionContext, base float64) float64 {
	players := activeRiskSeats(decision)
	if players <= 0 {
		return 1
	}
	refillWindow := float64(players * 6)
	finality := 1 - float64(decision.StockCount)/refillWindow
	if finality < 0 {
		finality = 0
	}
	if finality > 1 {
		finality = 1
	}
	if finality < base {
		return base
	}
	return finality
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

func cardStickiness(card domain.Card, trump domain.Suit, finality float64) float64 {
	if !validCard(card) {
		return 1
	}
	sticky := 0.65 + float64(card.Rank-domain.Six)*0.08
	if card.Suit == trump {
		sticky += 0.35 * (1 - finality)
	}
	if sticky < 0.2 {
		return 0.2
	}
	return sticky
}

func currentBattleThreat(decision *app.DecisionContext, hidden HiddenCards, seat domain.Seat) float64 {
	if decision.Phase != domain.MatchPhaseDefense && decision.Phase != domain.MatchPhaseTaking {
		return 0
	}
	if seat != decision.Defender {
		return 0
	}
	if seat == decision.Seat {
		return EvaluateBattleRisk(decision, hidden).Best
	}
	return float64(len(tableCardsForRisk(decision.Table))) * 0.8
}

func outletPotential(decision *app.DecisionContext, hand []domain.Card, seat domain.Seat) float64 {
	if seat != decision.Seat {
		return 0
	}
	outlet := 0.0
	for _, action := range decision.LegalActions {
		switch action.Kind {
		case domain.ActionKindAttack, domain.ActionKindThrowIn:
			outlet += 1.0 - math.Min(cardStickiness(action.Card, decision.TrumpSuit, 0), 1.4)/2
		case domain.ActionKindDefend, domain.ActionKindTransfer:
			outlet += 0.45
		case domain.ActionKindFinishDefense, domain.ActionKindFinishTake, domain.ActionKindPassThrowIn:
			outlet += 0.20
		}
	}
	if len(hand) > 1 {
		outlet += float64(rankPairs(hand)) * 0.25
	}
	return outlet
}

func defenseStability(decision *app.DecisionContext, hand []domain.Card) float64 {
	if len(hand) == 0 {
		return 0
	}
	stability := 0.0
	for _, attack := range likelyAttacks(decision) {
		if card, ok := cheapestDefenseCardForRisk(attack, hand, decision.TrumpSuit); ok {
			stability += 1 / (1 + float64(cardCost(card, decision.TrumpSuit))/20)
		}
	}
	return stability
}

func initiativeValue(decision *app.DecisionContext, seat domain.Seat) float64 {
	switch {
	case decision.Phase == domain.MatchPhaseAttack && seat == decision.Attacker:
		return 1.0
	case decision.Phase == domain.MatchPhaseThrowIn && seat != decision.Defender:
		return 0.45
	case decision.Phase == domain.MatchPhaseTaking && seat == decision.Defender:
		return -1.0
	case decision.Phase == domain.MatchPhaseDefense && seat == decision.Defender:
		return -0.60
	default:
		return 0
	}
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

func likelyAttacks(decision *app.DecisionContext) []domain.Card {
	cards := make([]domain.Card, 0, len(decision.Table)+len(decision.LegalActions))
	for _, pair := range decision.Table {
		if !pair.Defended && validCard(pair.Attack) {
			cards = append(cards, pair.Attack)
		}
	}
	for _, action := range decision.LegalActions {
		switch action.Kind {
		case domain.ActionKindAttack, domain.ActionKindThrowIn:
			if validCard(action.Card) {
				cards = append(cards, action.Card)
			}
		}
	}
	return cards
}

func cheapestDefenseCardForRisk(attack domain.Card, hand []domain.Card, trump domain.Suit) (domain.Card, bool) {
	var best domain.Card
	found := false
	for _, card := range hand {
		if !domain.CanBeat(attack, card, trump) {
			continue
		}
		if !found || cardCost(card, trump) < cardCost(best, trump) {
			best = card
			found = true
		}
	}
	return best, found
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
