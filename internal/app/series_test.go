package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSeriesStartsNextMatchBeforePreviousLoser(t *testing.T) {
	ctx := context.Background()
	series := mustSeries(t)
	firstDeck := deckForDeal(seriesHandsOne(), stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, seriesHandsOne()...))
	first, _, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID: "match-1",
		Deal:    fixedDeck(firstDeck),
	})
	if err != nil {
		t.Fatalf("StartMatch first returned error: %v", err)
	}
	err = first.Concede(ctx, domain.Seat(0))
	if err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	err = series.CompleteMatch(first)
	if err != nil {
		t.Fatalf("CompleteMatch returned error: %v", err)
	}

	internalEvents := app.NewInMemoryInternalEventStore()
	secondHands := seriesHandsTwo()
	secondDeck := deckForDeal(secondHands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, secondHands...))
	second, deal, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID:            "match-2",
		Deal:               fixedDeck(secondDeck),
		InternalEventStore: internalEvents,
	})
	if err != nil {
		t.Fatalf("StartMatch second returned error: %v", err)
	}

	if deal.FirstAttacker != 1 {
		t.Fatalf("second first attacker = %d, want seat before previous loser 1", deal.FirstAttacker)
	}
	if deal.RandomFirstAttacker {
		t.Fatal("RandomFirstAttacker = true, want false after series override")
	}
	view := second.ViewForSeat(domain.Seat(0))
	if view.Attacker != domain.Seat(1) || view.Defender != domain.Seat(0) {
		t.Fatalf("roles = %d/%d, want attacker 1 defender 0", view.Attacker, view.Defender)
	}
	events := internalEvents.EventsForMatch("match-2")
	if len(events) != 2 || events[1].Deal == nil {
		t.Fatalf("internal events = %+v, want canonical deal", events)
	}
	if events[1].Deal.FirstAttacker != domain.Seat(1) {
		t.Fatalf("internal first attacker = %d, want 1", events[1].Deal.FirstAttacker)
	}
	replay, err := app.ReplayInternalEvents(events, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("ReplayInternalEvents returned error: %v", err)
	}
	if replay.Match.Attacker() != domain.Seat(1) || replay.Match.Defender() != domain.Seat(0) {
		t.Fatalf("replay roles = %d/%d, want attacker 1 defender 0", replay.Match.Attacker(), replay.Match.Defender())
	}
}

func TestSeriesStartMatchEmitsConfigIdentity(t *testing.T) {
	ctx := context.Background()
	config, err := app.NewMatchConfig(app.RulePresetDefault, 3)
	if err != nil {
		t.Fatalf("NewMatchConfig returned error: %v", err)
	}
	identity, err := config.Identity()
	if err != nil {
		t.Fatalf("Identity returned error: %v", err)
	}
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-config",
		Config:   config,
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	if series.ConfigIdentity() != identity {
		t.Fatalf("series ConfigIdentity = %+v, want %+v", series.ConfigIdentity(), identity)
	}
	publicEvents := app.NewInMemoryEventStore()
	internalEvents := app.NewInMemoryInternalEventStore()

	_, _, err = series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID:            "match-config",
		EventStore:         publicEvents,
		InternalEventStore: internalEvents,
	})
	if err != nil {
		t.Fatalf("StartMatch returned error: %v", err)
	}

	events := publicEvents.EventsForMatch("match-config")
	if len(events) < 2 {
		t.Fatalf("public events = %+v, want started and deal", events)
	}
	if events[0].ConfigIdentity != identity {
		t.Fatalf("public started ConfigIdentity = %+v, want %+v", events[0].ConfigIdentity, identity)
	}
	if !events[1].ConfigIdentity.IsZero() {
		t.Fatalf("public deal ConfigIdentity = %+v, want zero", events[1].ConfigIdentity)
	}
	summary, err := app.BuildMatchSummary(events)
	if err != nil {
		t.Fatalf("BuildMatchSummary returned error: %v", err)
	}
	if summary.ConfigIdentity != identity {
		t.Fatalf("summary ConfigIdentity = %+v, want %+v", summary.ConfigIdentity, identity)
	}

	canonical := internalEvents.EventsForMatch("match-config")
	if len(canonical) < 2 {
		t.Fatalf("internal events = %+v, want started and deal", canonical)
	}
	if canonical[0].ConfigIdentity != identity {
		t.Fatalf("internal started ConfigIdentity = %+v, want %+v", canonical[0].ConfigIdentity, identity)
	}
	if !canonical[1].ConfigIdentity.IsZero() {
		t.Fatalf("internal deal ConfigIdentity = %+v, want zero", canonical[1].ConfigIdentity)
	}
}

func TestSeriesConfigCanDisablePreviousLoserOverride(t *testing.T) {
	ctx := context.Background()
	config, err := app.NewMatchConfig(app.RulePresetDefault, 2)
	if err != nil {
		t.Fatalf("NewMatchConfig returned error: %v", err)
	}
	config.Series.Consecutive = false
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{0, 1},
		Config:   config,
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}

	firstDeck := deckForDeal(seriesHandsOne(), stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, seriesHandsOne()...))
	first, _, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID: "match-1",
		Deal:    fixedDeck(firstDeck),
	})
	if err != nil {
		t.Fatalf("StartMatch first returned error: %v", err)
	}
	if concedeErr := first.Concede(ctx, domain.Seat(0)); concedeErr != nil {
		t.Fatalf("Concede returned error: %v", concedeErr)
	}
	if completeErr := series.CompleteMatch(first); completeErr != nil {
		t.Fatalf("CompleteMatch returned error: %v", completeErr)
	}

	secondHands := seriesHandsTwo()
	secondDeck := deckForDeal(secondHands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, secondHands...))
	_, deal, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID: "match-2",
		Deal:    fixedDeck(secondDeck),
	})
	if err != nil {
		t.Fatalf("StartMatch second returned error: %v", err)
	}
	if deal.FirstAttacker != 0 {
		t.Fatalf("second first attacker = %d, want deal-selected seat 0", deal.FirstAttacker)
	}
}

func TestSeriesClearsPreviousLoserWhenMatchHasNoLoser(t *testing.T) {
	ctx := context.Background()
	series := mustSeries(t)
	firstSession := mustSessionWithMatchID(t, "match-1", mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	if err := firstSession.Concede(ctx, domain.Seat(0)); err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	if err := series.CompleteMatch(firstSession); err != nil {
		t.Fatalf("CompleteMatch first returned error: %v", err)
	}
	if loser, ok := series.PreviousLoser(); !ok || loser != domain.Seat(0) {
		t.Fatalf("previous loser = %d/%v, want 0/true", loser, ok)
	}

	drawSession := mustSessionWithMatchID(t, "match-2", mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	mustApply(t, drawSession, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs}})
	mustApply(t, drawSession, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        domain.Card{Rank: domain.Seven, Suit: domain.Clubs},
		AttackIndex: 0,
	})
	mustApply(t, drawSession, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})
	if err := series.CompleteMatch(drawSession); err != nil {
		t.Fatalf("CompleteMatch draw returned error: %v", err)
	}

	if loser, ok := series.PreviousLoser(); ok || loser != domain.NoSeat {
		t.Fatalf("previous loser = %d/%v, want NoSeat/false", loser, ok)
	}
	results := series.Results()
	if len(results) != 2 || !results[1].Draw {
		t.Fatalf("results = %+v, want second draw", results)
	}
}

func TestSeriesSupportsThreeSeats(t *testing.T) {
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{0, 1, 2},
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	if !slices.Equal(series.Seats(), []domain.Seat{0, 1, 2}) {
		t.Fatalf("series seats = %v, want [0 1 2]", series.Seats())
	}
}

func TestThreeSeatSeriesStartsNextMatchBeforePreviousLoser(t *testing.T) {
	ctx := context.Background()
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{0, 1, 2},
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	first, _, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID: "match-1",
		Deal:    domain.SeededDealOptions(42),
	})
	if err != nil {
		t.Fatalf("StartMatch first returned error: %v", err)
	}
	if concedeErr := first.Concede(ctx, domain.Seat(0)); concedeErr != nil {
		t.Fatalf("Concede returned error: %v", concedeErr)
	}
	if completeErr := series.CompleteMatch(first); completeErr != nil {
		t.Fatalf("CompleteMatch returned error: %v", completeErr)
	}

	second, deal, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID: "match-2",
		Deal:    domain.SeededDealOptions(43),
	})
	if err != nil {
		t.Fatalf("StartMatch second returned error: %v", err)
	}

	if deal.FirstAttacker != 2 {
		t.Fatalf("second first attacker = %d, want seat before previous loser 2", deal.FirstAttacker)
	}
	view := second.ViewForSeat(domain.Seat(0))
	if view.Attacker != domain.Seat(2) || view.Defender != domain.Seat(0) {
		t.Fatalf("roles = %d/%d, want attacker 2 defender 0", view.Attacker, view.Defender)
	}
}

func TestSeriesRejectsUnsupportedSeatCount(t *testing.T) {
	_, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{0, 1, 2, 3, 4, 5, 6},
	})
	if !errors.Is(err, app.ErrInvalidSeries) {
		t.Fatalf("NewSeries error = %v, want ErrInvalidSeries", err)
	}
}

func TestSeriesRejectsNonCanonicalSeats(t *testing.T) {
	_, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{1, 0},
	})
	if !errors.Is(err, app.ErrInvalidSeries) {
		t.Fatalf("NewSeries error = %v, want ErrInvalidSeries", err)
	}
}

func TestSeriesRejectsConfigSeatMismatch(t *testing.T) {
	config, err := app.NewMatchConfig(app.RulePresetDefault, 3)
	if err != nil {
		t.Fatalf("NewMatchConfig returned error: %v", err)
	}
	_, err = app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{0, 1},
		Config:   config,
	})
	if !errors.Is(err, app.ErrInvalidSeries) {
		t.Fatalf("NewSeries error = %v, want ErrInvalidSeries", err)
	}
}

func TestSeriesRequiresCompletedMatch(t *testing.T) {
	series := mustSeries(t)
	session := mustSessionWithMatchID(t, "match-1", mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))

	err := series.CompleteMatch(session)
	if !errors.Is(err, app.ErrInvalidSeries) {
		t.Fatalf("CompleteMatch error = %v, want ErrInvalidSeries", err)
	}
}

func TestSeriesRejectsCompletedMatchIDReuse(t *testing.T) {
	ctx := context.Background()
	series := mustSeries(t)
	session := mustSessionWithMatchID(t, "match-1", mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	if err := session.Concede(ctx, domain.Seat(0)); err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	if err := series.CompleteMatch(session); err != nil {
		t.Fatalf("CompleteMatch returned error: %v", err)
	}

	if err := series.CompleteMatch(session); !errors.Is(err, app.ErrInvalidSeries) {
		t.Fatalf("CompleteMatch duplicate error = %v, want ErrInvalidSeries", err)
	}
	hands := seriesHandsOne()
	_, _, err := series.StartMatch(ctx, app.SeriesMatchOptions{
		MatchID: "match-1",
		Deal:    fixedDeck(deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))),
	})
	if !errors.Is(err, app.ErrInvalidSeries) {
		t.Fatalf("StartMatch duplicate error = %v, want ErrInvalidSeries", err)
	}
}

func mustSeries(t *testing.T) *app.Series {
	t.Helper()
	series, err := app.NewSeries(&app.SeriesOptions{
		SeriesID: "series-1",
		Seats:    []domain.Seat{0, 1},
	})
	if err != nil {
		t.Fatalf("NewSeries returned error: %v", err)
	}
	if !slices.Equal(series.Seats(), []domain.Seat{0, 1}) {
		t.Fatalf("series seats = %v, want [0 1]", series.Seats())
	}
	return series
}

func mustSessionWithMatchID(t *testing.T, matchID app.MatchID, match *domain.Match) *app.Session {
	t.Helper()
	session, err := app.NewSessionWithOptions(context.Background(), match, app.SessionOptions{MatchID: matchID})
	if err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}
	return session
}

func fixedDeck(deck []domain.Card) domain.DealOptions {
	return domain.DealOptions{
		Shuffler: domain.ShuffleFunc(func(cards []domain.Card) {
			copy(cards, deck)
		}),
	}
}

func seriesHandsOne() [][]domain.Card {
	return [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Eight, Suit: domain.Hearts},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Spades},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Hearts},
			{Rank: domain.Eight, Suit: domain.Spades},
		},
	}
}

func seriesHandsTwo() [][]domain.Card {
	return [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Hearts},
			{Rank: domain.Seven, Suit: domain.Clubs},
			{Rank: domain.Eight, Suit: domain.Diamonds},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Ten, Suit: domain.Hearts},
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Spades},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Eight, Suit: domain.Spades},
		},
	}
}
