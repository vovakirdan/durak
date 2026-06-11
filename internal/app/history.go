package app

import (
	"errors"
	"fmt"
	"slices"

	"github.com/vovakirdan/durak/internal/domain"
)

// ErrInvalidMatchHistory means a match event stream cannot form a read model.
var ErrInvalidMatchHistory = errors.New("invalid match history")

// MatchSummary is an event-derived read model for match lists and statistics.
type MatchSummary struct {
	MatchID           MatchID
	RuleProfile       string
	Seats             []domain.Seat
	InitialHandSizes  []int
	TrumpIndicator    domain.Card
	TrumpSuit         domain.Suit
	FirstAttacker     domain.Seat
	InitialDefender   domain.Seat
	InitialStockCount int
	ActionCount       int
	LastSequence      uint64
	Completed         bool
	Winner            domain.Seat
	Loser             domain.Seat
	Draw              bool
	ConcededBy        domain.Seat
}

// BuildMatchSummary projects one per-match event stream.
func BuildMatchSummary(events []Event) (MatchSummary, error) {
	if len(events) == 0 {
		return MatchSummary{}, fmt.Errorf("%w: event stream is empty", ErrInvalidMatchHistory)
	}
	summary := MatchSummary{
		MatchID:         events[0].MatchID,
		FirstAttacker:   domain.NoSeat,
		InitialDefender: domain.NoSeat,
		Winner:          domain.NoSeat,
		Loser:           domain.NoSeat,
		ConcededBy:      domain.NoSeat,
	}
	if summary.MatchID == "" {
		return MatchSummary{}, fmt.Errorf("%w: match id is empty", ErrInvalidMatchHistory)
	}
	for i := range events {
		if err := applySummaryEvent(&summary, &events[i]); err != nil {
			return MatchSummary{}, fmt.Errorf("%w: event %d: %w", ErrInvalidMatchHistory, i, err)
		}
	}
	return cloneMatchSummary(&summary), nil
}

// BuildMatchSummaries projects all events into per-match summaries in first-seen order.
func BuildMatchSummaries(events []Event) ([]MatchSummary, error) {
	groups := make(map[MatchID][]Event)
	order := make([]MatchID, 0)
	for i := range events {
		matchID := events[i].MatchID
		if matchID == "" {
			return nil, fmt.Errorf("%w: event %d match id is empty", ErrInvalidMatchHistory, i)
		}
		if _, ok := groups[matchID]; !ok {
			order = append(order, matchID)
		}
		groups[matchID] = append(groups[matchID], events[i])
	}

	summaries := make([]MatchSummary, 0, len(order))
	for _, matchID := range order {
		summary, err := BuildMatchSummary(groups[matchID])
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func applySummaryEvent(summary *MatchSummary, event *Event) error {
	if event.MatchID != summary.MatchID {
		return fmt.Errorf("match id = %q, want %q", event.MatchID, summary.MatchID)
	}
	if event.Sequence != summary.LastSequence+1 {
		return fmt.Errorf("sequence = %d, want %d", event.Sequence, summary.LastSequence+1)
	}
	summary.LastSequence = event.Sequence

	switch event.Domain.Kind {
	case domain.EventKindMatchStarted:
		if event.Domain.Started == nil {
			return missingHistoryPayload(event.Domain.Kind)
		}
		summary.RuleProfile = event.Domain.Started.RuleProfile
		return setSummarySeats(summary, event.Domain.Started.PlayerCount)
	case domain.EventKindDeal:
		if event.Domain.Deal == nil {
			return missingHistoryPayload(event.Domain.Kind)
		}
		return applySummaryDeal(summary, event.Domain.Deal)
	case domain.EventKindAttack, domain.EventKindDefend, domain.EventKindThrowIn,
		domain.EventKindTransfer, domain.EventKindTake, domain.EventKindFinishDefense,
		domain.EventKindFinishTake:
		if event.Domain.Action == nil {
			return missingHistoryPayload(event.Domain.Kind)
		}
		summary.ActionCount++
	case domain.EventKindConcede:
		if event.Domain.Concede == nil {
			return missingHistoryPayload(event.Domain.Kind)
		}
		summary.ConcededBy = event.Domain.Concede.Seat
	case domain.EventKindMatchEnded:
		if event.Domain.MatchEnded == nil {
			return missingHistoryPayload(event.Domain.Kind)
		}
		applySummaryResult(summary, event.Domain.MatchEnded)
	}
	return nil
}

func applySummaryDeal(summary *MatchSummary, deal *domain.DealEvent) error {
	if err := setSummarySeats(summary, len(deal.HandSizes)); err != nil {
		return err
	}
	summary.InitialHandSizes = slices.Clone(deal.HandSizes)
	summary.TrumpIndicator = deal.TrumpIndicator
	summary.TrumpSuit = deal.TrumpSuit
	summary.FirstAttacker = deal.FirstAttacker
	summary.InitialDefender = deal.Defender
	summary.InitialStockCount = deal.StockCount
	return nil
}

func applySummaryResult(summary *MatchSummary, result *domain.MatchEndedEvent) {
	summary.Completed = true
	summary.Winner = result.Winner
	summary.Loser = result.Loser
	summary.Draw = result.Draw
}

func setSummarySeats(summary *MatchSummary, playerCount int) error {
	if playerCount <= 0 {
		return fmt.Errorf("player count must be positive")
	}
	seats := make([]domain.Seat, playerCount)
	for i := range seats {
		seats[i] = domain.Seat(i)
	}
	if len(summary.Seats) != 0 && !slices.Equal(summary.Seats, seats) {
		return fmt.Errorf("seats = %v, want %v", seats, summary.Seats)
	}
	summary.Seats = seats
	return nil
}

func missingHistoryPayload(kind domain.EventKind) error {
	return fmt.Errorf("missing payload for event kind %d", kind)
}

func cloneMatchSummary(summary *MatchSummary) MatchSummary {
	if summary == nil {
		return MatchSummary{}
	}
	cloned := *summary
	cloned.Seats = slices.Clone(summary.Seats)
	cloned.InitialHandSizes = slices.Clone(summary.InitialHandSizes)
	return cloned
}
