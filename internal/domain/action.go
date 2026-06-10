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
	// ActionKindTake declares that the defender will take table cards.
	ActionKindTake
	// ActionKindFinishDefense completes a successful defense.
	ActionKindFinishDefense
	// ActionKindFinishTake completes taking cards after optional throw-ins.
	ActionKindFinishTake
)

// Action is a validated command candidate for a player seat.
type Action struct {
	Kind        ActionKind
	Seat        Seat
	Card        Card
	AttackIndex int
}

// ApplyAction validates and applies action through the same methods adapters use.
func (m *Match) ApplyAction(action Action) error {
	switch action.Kind {
	case ActionKindAttack:
		return m.Attack(action.Seat, action.Card)
	case ActionKindDefend:
		return m.Defend(action.Seat, action.AttackIndex, action.Card)
	case ActionKindThrowIn:
		return m.ThrowIn(action.Seat, action.Card)
	case ActionKindTake:
		return m.Take(action.Seat)
	case ActionKindFinishDefense:
		return m.FinishDefense(action.Seat)
	case ActionKindFinishTake:
		return m.FinishTake(action.Seat)
	default:
		return ErrInvalidAction
	}
}
