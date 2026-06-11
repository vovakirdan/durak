package app_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestBuildMatchSummarySupportsSeatSlices(t *testing.T) {
	events := historyEvents("match-1", 3, domain.Seat(2), domain.Seat(0))

	summary, err := app.BuildMatchSummary(events)
	if err != nil {
		t.Fatalf("BuildMatchSummary returned error: %v", err)
	}

	if summary.MatchID != "match-1" {
		t.Fatalf("MatchID = %q, want match-1", summary.MatchID)
	}
	if !slices.Equal(summary.Seats, []domain.Seat{0, 1, 2}) {
		t.Fatalf("Seats = %v, want [0 1 2]", summary.Seats)
	}
	if !slices.Equal(summary.InitialHandSizes, []int{6, 6, 6}) {
		t.Fatalf("InitialHandSizes = %v, want [6 6 6]", summary.InitialHandSizes)
	}
	if summary.FirstAttacker != domain.Seat(2) || summary.InitialDefender != domain.Seat(0) {
		t.Fatalf("roles = %d/%d, want 2/0", summary.FirstAttacker, summary.InitialDefender)
	}
	if summary.ActionCount != 1 {
		t.Fatalf("ActionCount = %d, want 1", summary.ActionCount)
	}
	if !summary.Completed || summary.Winner != domain.Seat(1) || summary.Loser != domain.Seat(0) || summary.Draw {
		t.Fatalf("result = %+v, want completed winner 1 loser 0", summary)
	}

	events[1].Domain.Deal.HandSizes[0] = 99
	if summary.InitialHandSizes[0] != 6 {
		t.Fatalf("mutating source events changed summary hand sizes: %+v", summary.InitialHandSizes)
	}
}

func TestBuildMatchSummariesKeepsFirstSeenMatchOrder(t *testing.T) {
	events := append(
		historyEvents("match-1", 2, domain.Seat(0), domain.Seat(1)),
		historyEvents("match-2", 3, domain.Seat(2), domain.Seat(0))...,
	)

	summaries, err := app.BuildMatchSummaries(events)
	if err != nil {
		t.Fatalf("BuildMatchSummaries returned error: %v", err)
	}

	if len(summaries) != 2 {
		t.Fatalf("summaries = %+v, want two", summaries)
	}
	if summaries[0].MatchID != "match-1" || summaries[1].MatchID != "match-2" {
		t.Fatalf("summary order = %q/%q, want match-1/match-2", summaries[0].MatchID, summaries[1].MatchID)
	}
	if !slices.Equal(summaries[1].Seats, []domain.Seat{0, 1, 2}) {
		t.Fatalf("second seats = %v, want [0 1 2]", summaries[1].Seats)
	}
}

func TestBuildMatchSummaryRejectsInvalidStreams(t *testing.T) {
	events := historyEvents("match-1", 2, domain.Seat(0), domain.Seat(1))
	events[2].MatchID = "other-match"

	_, err := app.BuildMatchSummary(events)

	if !errors.Is(err, app.ErrInvalidMatchHistory) {
		t.Fatalf("BuildMatchSummary error = %v, want ErrInvalidMatchHistory", err)
	}
}

func TestBuildMatchSummaryRejectsSeatCountMismatch(t *testing.T) {
	events := historyEvents("match-1", 3, domain.Seat(2), domain.Seat(0))
	events[1].Domain.Deal.HandSizes = []int{6, 6}

	_, err := app.BuildMatchSummary(events)

	if !errors.Is(err, app.ErrInvalidMatchHistory) {
		t.Fatalf("BuildMatchSummary error = %v, want ErrInvalidMatchHistory", err)
	}
}

func historyEvents(matchID app.MatchID, playerCount int, firstAttacker, defender domain.Seat) []app.Event {
	handSizes := make([]int, playerCount)
	for i := range handSizes {
		handSizes[i] = 6
	}
	return []app.Event{
		{
			MatchID:  matchID,
			Sequence: 1,
			Domain: domain.Event{
				Kind: domain.EventKindMatchStarted,
				Started: &domain.MatchStartedEvent{
					PlayerCount: playerCount,
					RuleProfile: "default",
				},
			},
		},
		{
			MatchID:  matchID,
			Sequence: 2,
			Domain: domain.Event{
				Kind: domain.EventKindDeal,
				Deal: &domain.DealEvent{
					TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
					TrumpSuit:      domain.Hearts,
					FirstAttacker:  firstAttacker,
					Defender:       defender,
					HandSizes:      handSizes,
					StockCount:     36 - playerCount*6,
				},
			},
		},
		{
			MatchID:  matchID,
			Sequence: 3,
			Domain: domain.Event{
				Kind: domain.EventKindAttack,
				Action: &domain.ActionEvent{Action: domain.Action{
					Kind: domain.ActionKindAttack,
					Seat: firstAttacker,
					Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs},
				}},
			},
		},
		{
			MatchID:  matchID,
			Sequence: 4,
			Domain: domain.Event{
				Kind: domain.EventKindMatchEnded,
				MatchEnded: &domain.MatchEndedEvent{
					Winner: domain.Seat(1),
					Loser:  domain.Seat(0),
				},
			},
		},
	}
}
