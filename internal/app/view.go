package app

import "github.com/vovakirdan/durak/internal/domain"

// SeatView is the non-secret match state visible to a seat.
type SeatView struct {
	Seat               domain.Seat
	Phase              domain.MatchPhase
	Attacker           domain.Seat
	Defender           domain.Seat
	TrumpSuit          domain.Suit
	TrumpIndicator     domain.Card
	Table              []domain.TablePair
	HandSizes          []int
	StockCount         int
	DiscardCount       int
	SuccessfulDefenses int
	Winner             domain.Seat
	Loser              domain.Seat
}

// DecisionContext is the read-only state a strategy may use for one seat.
type DecisionContext struct {
	SeatView
	Hand         []domain.Card
	LegalActions []domain.Action
	PublicMemory PublicCardMemory
}
