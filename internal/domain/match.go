package domain

import (
	"errors"
	"fmt"
	"slices"
)

var (
	// ErrInvalidMatch means the provided match state is internally inconsistent.
	ErrInvalidMatch = errors.New("invalid match")
	// ErrInvalidSeat means an action referenced a seat outside this match.
	ErrInvalidSeat = errors.New("invalid seat")
	// ErrMatchComplete means the match has already ended.
	ErrMatchComplete = errors.New("match complete")
	// ErrInvalidPhase means an action is not legal in the current phase.
	ErrInvalidPhase = errors.New("invalid match phase")
	// ErrNotPlayersTurn means the acting seat does not own the current action.
	ErrNotPlayersTurn = errors.New("not player's turn")
	// ErrCardNotInHand means the acting player does not hold the requested card.
	ErrCardNotInHand = errors.New("card not in hand")
	// ErrCardDoesNotBeat means a defense card cannot beat an attack card.
	ErrCardDoesNotBeat = errors.New("card does not beat attack")
	// ErrAttackAlreadyDefended means a table pair already has a defense card.
	ErrAttackAlreadyDefended = errors.New("attack already defended")
	// ErrAttackNotDefended means at least one attack card still lacks defense.
	ErrAttackNotDefended = errors.New("attack not defended")
)

// Seat identifies a player's place at the table.
type Seat int

// NoSeat marks an absent winner or loser.
const NoSeat Seat = -1

// MatchPhase identifies the current state-machine phase.
type MatchPhase uint8

const (
	// MatchPhaseUnknown is the zero value for an unset phase.
	MatchPhaseUnknown MatchPhase = iota
	// MatchPhaseAttack waits for the attacker to put a card on the table.
	MatchPhaseAttack
	// MatchPhaseDefense waits for the defender to beat or take cards.
	MatchPhaseDefense
	// MatchPhaseThrowIn means current attacks are beaten and may be finished.
	MatchPhaseThrowIn
	// MatchPhaseComplete means the match has ended.
	MatchPhaseComplete
)

// TablePair is one attack card and its optional defense card.
type TablePair struct {
	Attack   Card
	Defense  Card
	Defended bool
}

// Match owns mutable game state and validates all gameplay transitions.
type Match struct {
	profile            RuleProfile
	hands              [][]Card
	stock              []Card
	discard            []Card
	trumpSuit          Suit
	trumpIndicator     Card
	attacker           Seat
	defender           Seat
	phase              MatchPhase
	table              []TablePair
	successfulDefenses int
	winner             Seat
	loser              Seat
}

// NewMatch starts a match from an already validated initial deal.
func NewMatch(deal *InitialDeal, profile RuleProfile) (*Match, error) {
	if err := validateMatchInput(deal, profile); err != nil {
		return nil, err
	}

	attacker := Seat(deal.FirstAttacker)
	match := &Match{
		profile:        profile,
		hands:          cloneHands(deal.Hands),
		stock:          slices.Clone(deal.Stock),
		trumpSuit:      deal.TrumpSuit,
		trumpIndicator: deal.TrumpIndicator,
		attacker:       attacker,
		defender:       nextSeat(attacker, len(deal.Hands)),
		phase:          MatchPhaseAttack,
		winner:         NoSeat,
		loser:          NoSeat,
	}
	match.updateCompletion()
	return match, nil
}

func validateMatchInput(deal *InitialDeal, profile RuleProfile) error {
	if deal == nil {
		return fmt.Errorf("%w: initial deal is nil", ErrInvalidMatch)
	}
	if len(deal.Hands) != 2 {
		return fmt.Errorf("%w: match state machine currently supports 2 players", ErrInvalidPlayerCount)
	}
	if len(deal.Hands) > profile.MaxPlayers {
		return fmt.Errorf("%w: got %d, allowed up to %d", ErrInvalidPlayerCount, len(deal.Hands), profile.MaxPlayers)
	}
	if deal.FirstAttacker < 0 || deal.FirstAttacker >= len(deal.Hands) {
		return fmt.Errorf("%w: first attacker %d is outside seats", ErrInvalidMatch, deal.FirstAttacker)
	}
	if deal.TrumpSuit == SuitUnknown {
		return fmt.Errorf("%w: trump suit is unknown", ErrInvalidMatch)
	}
	return nil
}

func nextSeat(seat Seat, playerCount int) Seat {
	return Seat((int(seat) + 1) % playerCount)
}

// Phase returns the current state-machine phase.
func (m *Match) Phase() MatchPhase {
	return m.phase
}

// Attacker returns the seat that owns the current attack.
func (m *Match) Attacker() Seat {
	return m.attacker
}

// Defender returns the seat that currently defends.
func (m *Match) Defender() Seat {
	return m.defender
}

// TrumpSuit returns the match trump suit.
func (m *Match) TrumpSuit() Suit {
	return m.trumpSuit
}

// TrumpIndicator returns the visible trump indicator card.
func (m *Match) TrumpIndicator() Card {
	return m.trumpIndicator
}

// SuccessfulDefenses returns how many rounds ended with cards beaten.
func (m *Match) SuccessfulDefenses() int {
	return m.successfulDefenses
}

// Winner returns the winning seat, or NoSeat while the match is not decided.
func (m *Match) Winner() Seat {
	return m.winner
}

// Loser returns the losing seat, or NoSeat while the match is not decided.
func (m *Match) Loser() Seat {
	return m.loser
}

// Hand returns a copy of a player's current hand.
func (m *Match) Hand(seat Seat) []Card {
	if !m.validSeat(seat) {
		return nil
	}
	return slices.Clone(m.hands[int(seat)])
}

// Stock returns a copy of the draw stock in draw order.
func (m *Match) Stock() []Card {
	return slices.Clone(m.stock)
}

// Discard returns a copy of beaten cards.
func (m *Match) Discard() []Card {
	return slices.Clone(m.discard)
}

// Table returns a copy of cards currently on the table.
func (m *Match) Table() []TablePair {
	return slices.Clone(m.table)
}

func (m *Match) validSeat(seat Seat) bool {
	return seat >= 0 && int(seat) < len(m.hands)
}

func (m *Match) requireInProgress() error {
	if m.phase == MatchPhaseComplete {
		return ErrMatchComplete
	}
	return nil
}

func (m *Match) updateCompletion() {
	if len(m.stock) > 0 || len(m.table) > 0 {
		return
	}

	emptySeats := make([]Seat, 0, len(m.hands))
	nonEmptySeats := make([]Seat, 0, len(m.hands))
	for seat, hand := range m.hands {
		if len(hand) == 0 {
			emptySeats = append(emptySeats, Seat(seat))
			continue
		}
		nonEmptySeats = append(nonEmptySeats, Seat(seat))
	}
	if len(emptySeats) == 0 {
		return
	}

	m.phase = MatchPhaseComplete
	if len(nonEmptySeats) == 1 {
		m.winner = emptySeats[0]
		m.loser = nonEmptySeats[0]
	}
}
