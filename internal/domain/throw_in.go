package domain

import "fmt"

func (m *Match) startAttackRound(attacker Seat) {
	m.attackParticipants = m.attackParticipants[:0]
	m.addAttackParticipant(attacker)
	m.roundAttackLimit = m.attackLimitForDefender(m.defender)
	m.resetThrowInPasses()
	m.throwInLeadPending = m.profile.ThrowInOpening == ThrowInOpeningLeadFirst
}

func (m *Match) openThrowInWindow() {
	m.resetThrowInPasses()
	m.throwInLeadPending = m.profile.ThrowInOpening == ThrowInOpeningLeadFirst
}

func (m *Match) resetThrowInPasses() {
	if len(m.throwInPasses) != len(m.hands) {
		m.throwInPasses = make([]bool, len(m.hands))
		return
	}
	for seat := range m.throwInPasses {
		m.throwInPasses[seat] = false
	}
}

func (m *Match) closeAttackRound() {
	m.attackParticipants = nil
	m.resetThrowInPasses()
	m.throwInLeadPending = false
	m.roundAttackLimit = 0
}

func (m *Match) addAttackParticipant(seat Seat) {
	if !m.validSeat(seat) {
		return
	}
	for _, participant := range m.attackParticipants {
		if participant == seat {
			return
		}
	}
	m.attackParticipants = append(m.attackParticipants, seat)
}

func (m *Match) attackRefillOrder(defender Seat) []Seat {
	order := make([]Seat, 0, len(m.attackParticipants)+1)
	for _, seat := range m.attackParticipants {
		if seat == defender {
			continue
		}
		order = append(order, seat)
	}
	order = append(order, defender)
	return order
}

func (m *Match) attackLimitForDefender(defender Seat) int {
	if m.profile.AttackLimitPolicy != AttackLimitByDefenderInitialHand || !m.validSeat(defender) {
		return 0
	}
	return len(m.hands[int(defender)])
}

func (m *Match) validateCanAddAttackCard(defender Seat) error {
	return m.validateCanAddAttackCards(defender, 1)
}

func (m *Match) validateCanAddAttackCards(defender Seat, count int) error {
	if count <= 0 {
		return nil
	}
	if m.phase != MatchPhaseTaking && m.profile.FirstSuccessfulDefenseAttackLimit > 0 && m.successfulDefenses == 0 {
		limit := m.profile.FirstSuccessfulDefenseAttackLimit
		if len(m.table)+count > limit {
			return attackLimitError(limit)
		}
	}
	if m.profile.AttackLimitPolicy == AttackLimitByDefenderInitialHand {
		limit := m.roundAttackLimit
		if limit == 0 {
			limit = m.attackLimitForDefender(defender)
		}
		if len(m.table)+count > limit {
			return attackLimitError(limit)
		}
	}
	return nil
}

func (m *Match) throwInCandidateSeat(seat Seat) bool {
	if !m.activeSeat(seat) || seat == m.defender {
		return false
	}
	switch m.profile.ThrowInPlayerScope {
	case ThrowInPlayerScopeLeadOnly:
		return seat == m.attacker
	case ThrowInPlayerScopeNeighborsOnly:
		return seat == m.attacker || m.defenderNeighbor(seat)
	case ThrowInPlayerScopeAllExceptDefender:
		return true
	default:
		return seat == m.attacker
	}
}

func (m *Match) defenderNeighbor(seat Seat) bool {
	previous, hasPrevious := m.previousActiveSeatBefore(m.defender)
	next, hasNext := m.nextActiveSeatAfter(m.defender)
	return hasPrevious && seat == previous || hasNext && seat == next
}

func (m *Match) throwInOpeningAllows(seat Seat) bool {
	if m.profile.ThrowInOpening != ThrowInOpeningLeadFirst || !m.throwInLeadPending || seat == m.attacker {
		return true
	}
	return !m.seatHasThrowableCard(m.attacker)
}

func (m *Match) seatMayActInThrowInWindow(seat Seat) bool {
	if !m.throwInCandidateSeat(seat) || !m.throwInOpeningAllows(seat) {
		return false
	}
	return !m.throwInPasses[int(seat)]
}

func (m *Match) seatHasThrowableCard(seat Seat) bool {
	if !m.throwInCandidateSeat(seat) || !m.validSeat(seat) || m.throwInPasses[int(seat)] {
		return false
	}
	if err := m.validateCanAddAttackCard(m.defender); err != nil {
		return false
	}
	for _, card := range m.hands[int(seat)] {
		if m.rankOnTable(card.Rank) {
			return true
		}
	}
	return false
}

func (m *Match) canFinishThrowInWindow(finisher Seat) bool {
	if m.profile.ThrowInClose != ThrowInCloseAllEligiblePassed {
		return true
	}
	for seat := range m.hands {
		candidate := Seat(seat)
		if candidate == finisher || !m.throwInCandidateSeat(candidate) {
			continue
		}
		if m.seatHasThrowableCard(candidate) {
			return false
		}
	}
	return true
}

func attackLimitError(limit int) error {
	return fmt.Errorf("%w: attack limit is %d", ErrAttackLimitReached, limit)
}
