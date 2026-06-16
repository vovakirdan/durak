package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func newSummaryRow(summary *app.MatchSummary, projectedAt time.Time) (summaryRow, error) {
	if summary.ConfigIdentity.Hash == "" {
		return summaryRow{}, fmt.Errorf("%w: summary config hash is empty", ErrInvalidSQLiteStore)
	}
	seatsJSON, err := json.Marshal(summary.Seats)
	if err != nil {
		return summaryRow{}, err
	}
	handSizesJSON, err := json.Marshal(summary.InitialHandSizes)
	if err != nil {
		return summaryRow{}, err
	}
	return summaryRow{
		MatchID:              string(summary.MatchID),
		RuleProfile:          summary.RuleProfile,
		ConfigHash:           summary.ConfigIdentity.Hash,
		SeatsJSON:            string(seatsJSON),
		InitialHandSizesJSON: string(handSizesJSON),
		TrumpIndicatorRank:   int(summary.TrumpIndicator.Rank),
		TrumpIndicatorSuit:   int(summary.TrumpIndicator.Suit),
		TrumpSuit:            int(summary.TrumpSuit),
		FirstAttacker:        int(summary.FirstAttacker),
		InitialDefender:      int(summary.InitialDefender),
		InitialStockCount:    summary.InitialStockCount,
		ActionCount:          summary.ActionCount,
		LastSequence:         summary.LastSequence,
		Completed:            summary.Completed,
		Winner:               int(summary.Winner),
		Loser:                int(summary.Loser),
		Draw:                 summary.Draw,
		ConcededBy:           int(summary.ConcededBy),
		ProjectedAt:          projectedAt,
	}, nil
}

func rowSummary(row *summaryRow) (app.MatchSummary, error) {
	var seats []domain.Seat
	if err := json.Unmarshal([]byte(row.SeatsJSON), &seats); err != nil {
		return app.MatchSummary{}, fmt.Errorf("%w: decode summary seats: %w", ErrInvalidSQLiteStore, err)
	}
	var handSizes []int
	if err := json.Unmarshal([]byte(row.InitialHandSizesJSON), &handSizes); err != nil {
		return app.MatchSummary{}, fmt.Errorf("%w: decode summary hand sizes: %w", ErrInvalidSQLiteStore, err)
	}
	trumpIndicator, err := rowCard(row.TrumpIndicatorRank, row.TrumpIndicatorSuit)
	if err != nil {
		return app.MatchSummary{}, err
	}
	trumpSuit, err := rowSuit(row.TrumpSuit)
	if err != nil {
		return app.MatchSummary{}, err
	}
	return app.MatchSummary{
		MatchID: app.MatchID(row.MatchID),
		ConfigIdentity: app.MatchConfigIdentity{
			RuleProfile: row.RuleProfile,
			Hash:        row.ConfigHash,
		},
		RuleProfile:       row.RuleProfile,
		Seats:             seats,
		InitialHandSizes:  handSizes,
		TrumpIndicator:    trumpIndicator,
		TrumpSuit:         trumpSuit,
		FirstAttacker:     domain.Seat(row.FirstAttacker),
		InitialDefender:   domain.Seat(row.InitialDefender),
		InitialStockCount: row.InitialStockCount,
		ActionCount:       row.ActionCount,
		LastSequence:      row.LastSequence,
		Completed:         row.Completed,
		Winner:            domain.Seat(row.Winner),
		Loser:             domain.Seat(row.Loser),
		Draw:              row.Draw,
		ConcededBy:        domain.Seat(row.ConcededBy),
	}, nil
}

func rowCard(rankValue, suitValue int) (domain.Card, error) {
	rank, err := rowRank(rankValue)
	if err != nil {
		return domain.Card{}, err
	}
	suit, err := rowSuit(suitValue)
	if err != nil {
		return domain.Card{}, err
	}
	return domain.Card{Rank: rank, Suit: suit}, nil
}

func rowRank(value int) (domain.Rank, error) {
	if value < int(domain.Six) || value > int(domain.Ace) {
		return domain.RankUnknown, fmt.Errorf("%w: invalid rank %d", ErrInvalidSQLiteStore, value)
	}
	rank := domain.Rank(value)
	switch rank {
	case domain.Six, domain.Seven, domain.Eight, domain.Nine, domain.Ten,
		domain.Jack, domain.Queen, domain.King, domain.Ace:
		return rank, nil
	default:
		return domain.RankUnknown, fmt.Errorf("%w: invalid rank %d", ErrInvalidSQLiteStore, value)
	}
}

func rowSuit(value int) (domain.Suit, error) {
	if value < int(domain.Clubs) || value > int(domain.Spades) {
		return domain.SuitUnknown, fmt.Errorf("%w: invalid suit %d", ErrInvalidSQLiteStore, value)
	}
	suit := domain.Suit(value)
	switch suit {
	case domain.Clubs, domain.Diamonds, domain.Hearts, domain.Spades:
		return suit, nil
	default:
		return domain.SuitUnknown, fmt.Errorf("%w: invalid suit %d", ErrInvalidSQLiteStore, value)
	}
}
