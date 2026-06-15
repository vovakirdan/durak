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
	m.roundsStarted++
	m.phase = MatchPhaseDefense
	m.appendActionEvent(EventKindAttack, Action{Kind: ActionKindAttack, Seat: seat, Card: card})
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
	m.appendActionEvent(EventKindDefend, Action{
		Kind:        ActionKindDefend,
		Seat:        seat,
		Card:        defense,
		AttackIndex: attackIndex,
	})
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
	m.appendActionEvent(EventKindThrowIn, Action{Kind: ActionKindThrowIn, Seat: seat, Card: card})
	return nil
}

// Transfer adds a same-rank card and makes the previous attacker defend.
func (m *Match) Transfer(seat Seat, card Card) error {
	if err := m.validateTransfer(seat, card); err != nil {
		return err
	}

	if err := m.addAttackCard(seat, card); err != nil {
		return err
	}
	if nextDefender, ok := m.nextActiveSeatAfter(seat); ok {
		m.attacker = seat
		m.defender = nextDefender
	}
	m.appendActionEvent(EventKindTransfer, Action{Kind: ActionKindTransfer, Seat: seat, Card: card})
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

	cards := tableCards(m.table)
	m.appendActionEvent(EventKindFinishDefense, Action{Kind: ActionKindFinishDefense, Seat: seat})
	m.discard = append(m.discard, cards...)
	m.table = nil
	m.successfulDefenses++

	oldAttacker := m.attacker
	oldDefender := m.defender
	m.refill(oldAttacker, oldDefender)
	completed := m.completeIfFinished()
	if !completed {
		m.setNextRolesFrom(oldDefender)
		m.phase = MatchPhaseAttack
	}
	m.appendRoundEndedEvent(RoundOutcomeDefense, oldAttacker, oldDefender, cards)
	if completed {
		m.appendMatchEndedEvent()
	}
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
	m.appendActionEvent(EventKindTake, Action{Kind: ActionKindTake, Seat: seat})
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

	cards := tableCards(m.table)
	m.appendActionEvent(EventKindFinishTake, Action{Kind: ActionKindFinishTake, Seat: seat})
	m.hands[int(m.defender)] = append(m.hands[int(m.defender)], cards...)
	m.table = nil

	oldAttacker := m.attacker
	oldDefender := m.defender
	m.refill(oldAttacker, oldDefender)
	completed := m.completeIfFinished()
	if !completed {
		m.setNextRolesAfter(oldDefender)
		m.phase = MatchPhaseAttack
	}
	m.appendRoundEndedEvent(RoundOutcomeTake, oldAttacker, oldDefender, cards)
	if completed {
		m.appendMatchEndedEvent()
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

func (m *Match) refill(order ...Seat) {
	seen := make(map[Seat]bool, len(order))
	for _, seat := range order {
		if seen[seat] || !m.validSeat(seat) {
			continue
		}
		seen[seat] = true
		drawn := m.drawUpTo(seat, m.profile.InitialHandSize)
		if drawn > 0 {
			m.appendEvent(Event{
				Kind: EventKindRefill,
				Refill: &RefillEvent{
					Seat:       seat,
					Drawn:      drawn,
					HandSize:   len(m.hands[int(seat)]),
					StockCount: len(m.stock),
				},
			})
		}
	}
}

func (m *Match) drawUpTo(seat Seat, handSize int) int {
	drawn := 0
	for len(m.stock) > 0 && len(m.hands[int(seat)]) < handSize {
		m.hands[int(seat)] = append(m.hands[int(seat)], m.stock[0])
		m.stock = m.stock[1:]
		drawn++
	}
	return drawn
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
