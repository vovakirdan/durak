package domain

// LegalActions returns deterministic action candidates for a seat.
func (m *Match) LegalActions(seat Seat) []Action {
	if !m.validSeat(seat) || m.phase == MatchPhaseComplete {
		return nil
	}

	switch m.phase {
	case MatchPhaseAttack:
		return m.legalAttackActions(seat)
	case MatchPhaseDefense:
		return m.legalDefenseActions(seat)
	case MatchPhaseThrowIn:
		return m.legalThrowInActions(seat, true)
	case MatchPhaseTaking:
		return m.legalThrowInActions(seat, false)
	default:
		return nil
	}
}

func (m *Match) legalAttackActions(seat Seat) []Action {
	if seat != m.attacker {
		return nil
	}

	actions := make([]Action, 0, len(m.hands[int(seat)]))
	for _, card := range m.hands[int(seat)] {
		actions = append(actions, Action{
			Kind: ActionKindAttack,
			Seat: seat,
			Card: card,
		})
	}
	return actions
}

func (m *Match) legalDefenseActions(seat Seat) []Action {
	if seat != m.defender {
		return nil
	}

	actions := make([]Action, 0, len(m.hands[int(seat)])*2+1)
	for attackIndex, pair := range m.table {
		if pair.Defended {
			continue
		}
		for _, card := range m.hands[int(seat)] {
			if !CanBeat(pair.Attack, card, m.trumpSuit) {
				continue
			}
			actions = append(actions, Action{
				Kind:        ActionKindDefend,
				Seat:        seat,
				Card:        card,
				AttackIndex: attackIndex,
			})
		}
	}
	for _, card := range m.hands[int(seat)] {
		if m.validateTransfer(seat, card) != nil {
			continue
		}
		actions = append(actions, Action{
			Kind: ActionKindTransfer,
			Seat: seat,
			Card: card,
		})
	}
	actions = append(actions, Action{Kind: ActionKindTake, Seat: seat})
	return actions
}

func (m *Match) legalThrowInActions(seat Seat, includeFinishDefense bool) []Action {
	if !m.throwInCandidateSeat(seat) {
		return nil
	}

	actions := make([]Action, 0, len(m.hands[int(seat)])+2)
	for _, card := range m.hands[int(seat)] {
		if m.validateThrowIn(seat, card) != nil {
			continue
		}
		actions = append(actions, Action{
			Kind: ActionKindThrowIn,
			Seat: seat,
			Card: card,
		})
	}
	if m.validatePassThrowIn(seat) == nil {
		actions = append(actions, Action{Kind: ActionKindPassThrowIn, Seat: seat})
	}
	if seat == m.attacker && m.canFinishThrowInWindow(seat) {
		if includeFinishDefense {
			actions = append(actions, Action{Kind: ActionKindFinishDefense, Seat: seat})
		} else {
			actions = append(actions, Action{Kind: ActionKindFinishTake, Seat: seat})
		}
	}
	return actions
}
