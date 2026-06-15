package domain

import "fmt"

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
	if !m.seatMayActInThrowInWindow(seat) {
		return fmt.Errorf("%w: throw-in is not available for seat %d", ErrNotPlayersTurn, seat)
	}
	if !m.hasCard(seat, card) {
		return fmt.Errorf("%w: %s", ErrCardNotInHand, card)
	}
	if !m.rankOnTable(card.Rank) {
		return fmt.Errorf("%w: %s", ErrThrowInRankUnavailable, card)
	}
	if err := m.validateCanAddAttackCard(m.defender); err != nil {
		return err
	}
	return nil
}

func (m *Match) validatePassThrowIn(seat Seat) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if m.phase != MatchPhaseThrowIn && m.phase != MatchPhaseTaking {
		return fmt.Errorf("%w: pass requires throw-in or taking phase", ErrInvalidPhase)
	}
	if !m.seatMayActInThrowInWindow(seat) {
		return fmt.Errorf("%w: throw-in pass is not available for seat %d", ErrNotPlayersTurn, seat)
	}
	if !m.seatHasThrowableCard(seat) {
		return fmt.Errorf("%w: no throw-in card is available for seat %d", ErrThrowInRankUnavailable, seat)
	}
	if m.throwInPasses[int(seat)] {
		return fmt.Errorf("%w: seat %d", ErrThrowInAlreadyPassed, seat)
	}
	return nil
}

func (m *Match) validateTransfer(seat Seat, card Card) error {
	if err := m.requireInProgress(); err != nil {
		return err
	}
	if !m.profile.TransferEnabled {
		return ErrTransferDisabled
	}
	if m.phase != MatchPhaseDefense {
		return fmt.Errorf("%w: transfer requires defense phase", ErrInvalidPhase)
	}
	if seat != m.defender {
		return fmt.Errorf("%w: defender is %d", ErrNotPlayersTurn, m.defender)
	}
	if !m.profile.FirstAttackTransferAllowed && m.roundsStarted == 1 {
		return fmt.Errorf("%w: first attack cannot be transferred", ErrTransferNotAllowed)
	}
	if m.hasDefendedTableCards() {
		return ErrTransferAfterDefense
	}
	if !m.hasCard(seat, card) {
		return fmt.Errorf("%w: %s", ErrCardNotInHand, card)
	}
	if !m.rankOnTable(card.Rank) {
		return fmt.Errorf("%w: %s", ErrTransferRankUnavailable, card)
	}
	nextDefender, ok := m.nextActiveSeatAfter(seat)
	if !ok {
		return fmt.Errorf("%w: no next active defender", ErrTransferNotAllowed)
	}
	if err := m.validateCanAddAttackCard(nextDefender); err != nil {
		return err
	}
	return nil
}

func (m *Match) hasDefendedTableCards() bool {
	for _, pair := range m.table {
		if pair.Defended {
			return true
		}
	}
	return false
}

func (m *Match) rankOnTable(rank Rank) bool {
	for _, pair := range m.table {
		if pair.Attack.Rank == rank || pair.Defended && pair.Defense.Rank == rank {
			return true
		}
	}
	return false
}
