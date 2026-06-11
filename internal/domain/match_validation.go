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
	if !m.canAddSuccessfulDefenseAttack() {
		return fmt.Errorf("%w: first successful defense attack limit is %d", ErrAttackLimitReached, m.firstSuccessfulDefenseLimit())
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
