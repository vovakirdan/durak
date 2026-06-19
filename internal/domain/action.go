package domain

// ActionKind identifies a player command that can mutate a match.
type ActionKind uint8

const (
	// ActionKindUnknown is the zero value for an unset action kind.
	ActionKindUnknown ActionKind = iota
	// ActionKindAttack puts the first attack card on the table.
	ActionKindAttack
	// ActionKindDefend beats one attack card.
	ActionKindDefend
	// ActionKindThrowIn adds a legal attack card after ranks appear on the table.
	ActionKindThrowIn
	// ActionKindPassThrowIn declines the current optional throw-in window.
	ActionKindPassThrowIn
	// ActionKindTake declares that the defender will take table cards.
	ActionKindTake
	// ActionKindFinishDefense completes a successful defense.
	ActionKindFinishDefense
	// ActionKindFinishTake completes taking cards after optional throw-ins.
	ActionKindFinishTake
	// ActionKindTransfer passes the current defense to the previous attacker.
	ActionKindTransfer
)

const maxActionAttackCards = 4

// Action is a validated command candidate for a player seat.
type Action struct {
	Kind        ActionKind
	Seat        Seat
	Card        Card
	Cards       [maxActionAttackCards]Card
	CardCount   int
	AttackIndex int
}

// NewAttackAction builds either a legacy one-card attack or a packet attack.
func NewAttackAction(seat Seat, cards ...Card) Action {
	action := Action{Kind: ActionKindAttack, Seat: seat}
	if len(cards) == 0 {
		return action
	}
	action.Card = cards[0]
	if len(cards) == 1 {
		return action
	}
	count := min(len(cards), len(action.Cards))
	copy(action.Cards[:], cards[:count])
	action.CardCount = count
	return action
}

// AttackCards returns the attack packet, falling back to legacy Action.Card.
func (a Action) AttackCards() []Card {
	if a.Kind != ActionKindAttack {
		return nil
	}
	if a.CardCount == 0 {
		if a.Card == (Card{}) {
			return nil
		}
		return []Card{a.Card}
	}
	count := min(int(a.CardCount), len(a.Cards))
	cards := make([]Card, count)
	copy(cards, a.Cards[:count])
	return cards
}

// ApplyAction validates and applies action through the same methods adapters use.
func (m *Match) ApplyAction(action Action) error {
	switch action.Kind {
	case ActionKindAttack:
		return m.AttackMany(action.Seat, action.AttackCards())
	case ActionKindDefend:
		return m.Defend(action.Seat, action.AttackIndex, action.Card)
	case ActionKindThrowIn:
		return m.ThrowIn(action.Seat, action.Card)
	case ActionKindPassThrowIn:
		return m.PassThrowIn(action.Seat)
	case ActionKindTake:
		return m.Take(action.Seat)
	case ActionKindFinishDefense:
		return m.FinishDefense(action.Seat)
	case ActionKindFinishTake:
		return m.FinishTake(action.Seat)
	case ActionKindTransfer:
		return m.Transfer(action.Seat, action.Card)
	default:
		return ErrInvalidAction
	}
}
