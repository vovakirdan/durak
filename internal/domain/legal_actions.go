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

	hand := m.hands[int(seat)]
	actions := make([]Action, 0, len(hand))
	groups := make([]attackRankGroup, 0, len(hand))
	for _, card := range m.hands[int(seat)] {
		actions = append(actions, NewAttackAction(seat, card))
		groups = appendAttackRankGroup(groups, card)
	}
	limit := m.attackPacketLimit()
	for _, group := range groups {
		maxCount := min(len(group.cards), limit, maxActionAttackCards)
		for count := 2; count <= maxCount; count++ {
			actions = appendAttackCombinations(actions, seat, group.cards, count)
		}
	}
	return actions
}

type attackRankGroup struct {
	rank  Rank
	cards []Card
}

func appendAttackRankGroup(groups []attackRankGroup, card Card) []attackRankGroup {
	for index := range groups {
		if groups[index].rank == card.Rank {
			groups[index].cards = append(groups[index].cards, card)
			return groups
		}
	}
	return append(groups, attackRankGroup{rank: card.Rank, cards: []Card{card}})
}

func appendAttackCombinations(actions []Action, seat Seat, cards []Card, count int) []Action {
	if count <= 1 || count > len(cards) {
		return actions
	}
	packet := make([]Card, count)
	var search func(start int, depth int)
	search = func(start int, depth int) {
		if depth == count {
			actions = append(actions, NewAttackAction(seat, packet...))
			return
		}
		remaining := count - depth
		for index := start; index <= len(cards)-remaining; index++ {
			packet[depth] = cards[index]
			search(index+1, depth+1)
		}
	}
	search(0, 0)
	return actions
}

func (m *Match) attackPacketLimit() int {
	limit := maxActionAttackCards
	if m.profile.AttackLimitPolicy == AttackLimitByDefenderInitialHand && m.validSeat(m.defender) {
		limit = min(limit, m.attackLimitForDefender(m.defender))
	}
	if m.profile.FirstSuccessfulDefenseAttackLimit > 0 && m.successfulDefenses == 0 {
		limit = min(limit, m.profile.FirstSuccessfulDefenseAttackLimit-len(m.table))
	}
	if limit < 0 {
		return 0
	}
	return limit
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
