package domain

import (
	"fmt"
	"slices"
)

// Attack starts an attack with one card from the current attacker.
func (m *Match) Attack(seat Seat, card Card) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseAttack {
		return fmt.Errorf("%w: attack is allowed only in attack phase", ErrInvalidPhase)
	}
	if seat != m.attacker {
		return fmt.Errorf("%w: attacker is %d", ErrNotPlayersTurn, m.attacker)
	}
	if err := m.removeFromHand(seat, card); err != nil {
		return err
	}

	m.table = append(m.table, TablePair{Attack: card})
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

// FinishDefense moves fully defended table cards to discard and advances roles.
func (m *Match) FinishDefense(seat Seat) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseThrowIn {
		return fmt.Errorf("%w: all attacks must be defended before finishing", ErrInvalidPhase)
	}
	if seat != m.attacker {
		return fmt.Errorf("%w: attacker is %d", ErrNotPlayersTurn, m.attacker)
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

// Take gives table cards to the defender and advances to the next attack.
func (m *Match) Take(seat Seat) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseDefense && m.phase != MatchPhaseThrowIn {
		return fmt.Errorf("%w: take requires cards on the table", ErrInvalidPhase)
	}
	if seat != m.defender {
		return fmt.Errorf("%w: defender is %d", ErrNotPlayersTurn, m.defender)
	}

	m.hands[int(seat)] = append(m.hands[int(seat)], tableCards(m.table)...)
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
