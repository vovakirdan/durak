package domain

import (
	"fmt"
	"slices"
)

const attackerTurnErrorFormat = "%w: attacker is %d"

// Attack starts an attack with one card from the current attacker.
func (m *Match) Attack(seat Seat, card Card) error {
	if err := m.validateAttack(seat, card); err != nil {
		return err
	}

	if err := m.addAttackCard(seat, card); err != nil {
		return err
	}
	m.phase = MatchPhaseDefense
	return nil
}

// Defend beats one attack card with a card from the current defender.
func (m *Match) Defend(seat Seat, attackIndex int, defense Card) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseDefense {
		return fmt.Errorf("%w: defense is allowed only in defense phase", ErrInvalidPhase)
	}
	if seat != m.defender {
		return fmt.Errorf("%w: defender is %d", ErrNotPlayersTurn, m.defender)
	}
	if attackIndex < 0 || attackIndex >= len(m.table) {
		return fmt.Errorf("%w: attack index %d is outside table", ErrInvalidMatch, attackIndex)
	}

	pair := &m.table[attackIndex]
	if pair.Defended {
		return fmt.Errorf("%w: attack index %d", ErrAttackAlreadyDefended, attackIndex)
	}
	if !CanBeat(pair.Attack, defense, m.trumpSuit) {
		return fmt.Errorf("%w: %s does not beat %s", ErrCardDoesNotBeat, defense, pair.Attack)
	}
	if err := m.removeFromHand(seat, defense); err != nil {
		return err
	}

	pair.Defense = defense
	pair.Defended = true
	if m.allAttacksDefended() {
		m.phase = MatchPhaseThrowIn
	}
	return nil
}

// ThrowIn adds a legal attack card after ranks are present on the table.
func (m *Match) ThrowIn(seat Seat, card Card) error {
	if err := m.validateThrowIn(seat, card); err != nil {
		return err
	}

	if err := m.addAttackCard(seat, card); err != nil {
		return err
	}
	if m.phase == MatchPhaseThrowIn {
		m.phase = MatchPhaseDefense
	}
	return nil
}

// FinishDefense moves fully defended table cards to discard and advances roles.
func (m *Match) FinishDefense(seat Seat) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseThrowIn {
		return fmt.Errorf("%w: all attacks must be defended before finishing", ErrInvalidPhase)
	}
	if seat != m.attacker {
		return fmt.Errorf(attackerTurnErrorFormat, ErrNotPlayersTurn, m.attacker)
	}
	if !m.allAttacksDefended() {
		return ErrAttackNotDefended
	}

	m.discard = append(m.discard, tableCards(m.table)...)
	m.table = nil
	m.successfulDefenses++

	oldAttacker := m.attacker
	oldDefender := m.defender
	m.refill(oldAttacker, oldDefender)
	m.attacker = oldDefender
	m.defender = nextSeat(m.attacker, len(m.hands))
	m.phase = MatchPhaseAttack
	m.updateCompletion()
	return nil
}

// Take declares that the defender will take cards after optional throw-ins.
func (m *Match) Take(seat Seat) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseDefense {
		return fmt.Errorf("%w: take requires an active defense", ErrInvalidPhase)
	}
	if seat != m.defender {
		return fmt.Errorf("%w: defender is %d", ErrNotPlayersTurn, m.defender)
	}

	m.phase = MatchPhaseTaking
	return nil
}

// FinishTake gives table cards to the defender and advances to the next attack.
func (m *Match) FinishTake(seat Seat) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseTaking {
		return fmt.Errorf("%w: finish take requires taking phase", ErrInvalidPhase)
	}
	if seat != m.attacker {
		return fmt.Errorf(attackerTurnErrorFormat, ErrNotPlayersTurn, m.attacker)
	}

	m.hands[int(m.defender)] = append(m.hands[int(m.defender)], tableCards(m.table)...)
	m.table = nil

	oldAttacker := m.attacker
	oldDefender := m.defender
	m.refill(oldAttacker, oldDefender)
	m.attacker = nextSeat(oldDefender, len(m.hands))
	m.defender = nextSeat(m.attacker, len(m.hands))
	m.phase = MatchPhaseAttack
	m.updateCompletion()
	return nil
}

func (m *Match) validateAttack(seat Seat, card Card) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseAttack {
		return fmt.Errorf("%w: attack is allowed only in attack phase", ErrInvalidPhase)
	}
	if seat != m.attacker {
		return fmt.Errorf(attackerTurnErrorFormat, ErrNotPlayersTurn, m.attacker)
	}
	if !m.hasCard(seat, card) {
		return fmt.Errorf("%w: %s", ErrCardNotInHand, card)
	}
	return nil
}

func (m *Match) validateThrowIn(seat Seat, card Card) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseThrowIn && m.phase != MatchPhaseTaking {
		return fmt.Errorf("%w: throw-in requires throw-in or taking phase", ErrInvalidPhase)
	}
	if seat != m.attacker {
		return fmt.Errorf(attackerTurnErrorFormat, ErrNotPlayersTurn, m.attacker)
	}
	if !m.hasCard(seat, card) {
		return fmt.Errorf("%w: %s", ErrCardNotInHand, card)
	}
	if !m.rankOnTable(card.Rank) {
		return fmt.Errorf("%w: %s", ErrThrowInRankUnavailable, card)
	}
	if m.phase != MatchPhaseTaking && !m.canAddSuccessfulDefenseAttack() {
		return fmt.Errorf("%w: first successful defense attack limit is %d", ErrAttackLimitReached, m.firstSuccessfulDefenseLimit())
	}
	return nil
}

func (m *Match) addAttackCard(seat Seat, card Card) error {
	if err := m.removeFromHand(seat, card); err != nil {
		return err
	}
	m.table = append(m.table, TablePair{Attack: card})
	return nil
}

func (m *Match) removeFromHand(seat Seat, card Card) error {
	if !m.validSeat(seat) {
		return fmt.Errorf("%w: %d", ErrInvalidSeat, seat)
	}
	index := slices.Index(m.hands[int(seat)], card)
	if index == -1 {
		return fmt.Errorf("%w: %s", ErrCardNotInHand, card)
	}
	m.hands[int(seat)] = slices.Delete(m.hands[int(seat)], index, index+1)
	return nil
}

func (m *Match) hasCard(seat Seat, card Card) bool {
	if !m.validSeat(seat) {
		return false
	}
	return slices.Contains(m.hands[int(seat)], card)
}

func (m *Match) allAttacksDefended() bool {
	if len(m.table) == 0 {
		return false
	}
	for _, pair := range m.table {
		if !pair.Defended {
			return false
		}
	}
	return true
}

func (m *Match) rankOnTable(rank Rank) bool {
	for _, pair := range m.table {
		if pair.Attack.Rank == rank || pair.Defended && pair.Defense.Rank == rank {
			return true
		}
	}
	return false
}

func (m *Match) canAddSuccessfulDefenseAttack() bool {
	limit := m.firstSuccessfulDefenseLimit()
	return limit == 0 || len(m.table) < limit
}

func (m *Match) firstSuccessfulDefenseLimit() int {
	if m.successfulDefenses > 0 || m.profile.FirstSuccessfulDefenseAttackLimit <= 0 {
		return 0
	}
	return m.profile.FirstSuccessfulDefenseAttackLimit
}

func (m *Match) refill(order ...Seat) {
	seen := make(map[Seat]bool, len(order))
	for _, seat := range order {
		if seen[seat] || !m.validSeat(seat) {
			continue
		}
		seen[seat] = true
		m.drawUpTo(seat, m.profile.InitialHandSize)
	}
}

func (m *Match) drawUpTo(seat Seat, handSize int) {
	for len(m.stock) > 0 && len(m.hands[int(seat)]) < handSize {
		m.hands[int(seat)] = append(m.hands[int(seat)], m.stock[0])
		m.stock = m.stock[1:]
	}
}

func tableCards(table []TablePair) []Card {
	cards := make([]Card, 0, len(table)*2)
	for _, pair := range table {
		cards = append(cards, pair.Attack)
		if pair.Defended {
			cards = append(cards, pair.Defense)
		}
	}
	return cards
}
