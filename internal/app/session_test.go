package app_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSessionDecisionContextReturnsCopies(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	match := mustMatch(t, [][]domain.Card{
		{attack},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	})
	session := mustSession(t, match)

	decision := session.DecisionContext(domain.Seat(0))
	if !slices.Equal(decision.Hand, []domain.Card{attack}) {
		t.Fatalf("Hand = %v, want attacker card", decision.Hand)
	}
	if !slices.Equal(decision.HandSizes, []int{1, 1}) {
		t.Fatalf("HandSizes = %v, want [1 1]", decision.HandSizes)
	}
	if !slices.Contains(decision.LegalActions, domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: attack,
	}) {
		t.Fatalf("LegalActions = %v, want attack action", decision.LegalActions)
	}

	decision.Hand[0] = domain.Card{Rank: domain.Ace, Suit: domain.Spades}
	decision.HandSizes[0] = 99
	decision.LegalActions[0].Card = domain.Card{Rank: domain.Ace, Suit: domain.Spades}

	next := session.DecisionContext(domain.Seat(0))
	if !slices.Equal(next.Hand, []domain.Card{attack}) {
		t.Fatalf("mutating decision hand leaked into session: %v", next.Hand)
	}
	if !slices.Equal(next.HandSizes, []int{1, 1}) {
		t.Fatalf("mutating hand sizes leaked into session: %v", next.HandSizes)
	}
	if next.LegalActions[0].Card != attack {
		t.Fatalf("mutating legal actions leaked into session: %v", next.LegalActions)
	}
}

func TestSessionApplyStrategyAppliesChosenLegalAction(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{attack},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	strategy := strategyFunc(func(_ context.Context, decision *app.DecisionContext) (domain.Action, error) {
		return decision.LegalActions[0], nil
	})

	action, err := session.ApplyStrategy(context.Background(), domain.Seat(0), strategy)
	if err != nil {
		t.Fatalf("ApplyStrategy returned error: %v", err)
	}

	if action.Kind != domain.ActionKindAttack || action.Card != attack {
		t.Fatalf("action = %v, want attack with %v", action, attack)
	}
	if got := session.ViewForSeat(domain.Seat(0)).Phase; got != domain.MatchPhaseDefense {
		t.Fatalf("Phase = %v, want MatchPhaseDefense", got)
	}
}

func TestSessionApplyStrategyRejectsIllegalAction(t *testing.T) {
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))
	strategy := strategyFunc(func(context.Context, *app.DecisionContext) (domain.Action, error) {
		return domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)}, nil
	})

	_, err := session.ApplyStrategy(context.Background(), domain.Seat(0), strategy)
	if !errors.Is(err, app.ErrIllegalAction) {
		t.Fatalf("ApplyStrategy error = %v, want ErrIllegalAction", err)
	}
	if got := session.ViewForSeat(domain.Seat(0)).Phase; got != domain.MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", got)
	}
}

func TestSessionWithEventSinkEmitsInitialEvents(t *testing.T) {
	recorder := app.NewInMemoryEventRecorder()
	session, err := app.NewSessionWithOptions(context.Background(), mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}), app.SessionOptions{EventSink: recorder})
	if err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}

	if session.ViewForSeat(domain.Seat(0)).Phase != domain.MatchPhaseAttack {
		t.Fatalf("session was not created")
	}
	events := recorder.Events()
	expectEventKinds(t, events, domain.EventKindMatchStarted, domain.EventKindDeal)
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("sequences = %d/%d, want 1/2", events[0].Sequence, events[1].Sequence)
	}
	if got := events[1].Domain.Deal.HandSizes; !slices.Equal(got, []int{1, 1}) {
		t.Fatalf("deal hand sizes = %v, want [1 1]", got)
	}
}

func TestSessionApplyActionEmitsSequencedEvents(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	recorder := app.NewInMemoryEventRecorder()
	session := mustSessionWithSink(t, mustMatch(t, [][]domain.Card{
		{attack},
		{defense},
	}), recorder)

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	events := recorder.Events()
	expectEventKinds(t, events,
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindAttack,
		domain.EventKindDefend,
		domain.EventKindFinishDefense,
		domain.EventKindRoundEnded,
		domain.EventKindMatchEnded,
	)
	for i, event := range events {
		if event.Sequence != uint64(i+1) {
			t.Fatalf("event %d sequence = %d, want %d", i, event.Sequence, i+1)
		}
	}
	if events[5].Domain.RoundEnded.Outcome != domain.RoundOutcomeDefense {
		t.Fatalf("round outcome = %v, want defense", events[5].Domain.RoundEnded.Outcome)
	}
	if !events[6].Domain.MatchEnded.Draw {
		t.Fatalf("match ended draw = false, want true")
	}
}

func TestSessionEmitsRefillEvents(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	recorder := app.NewInMemoryEventRecorder()
	session := mustSessionWithSink(t, mustMatchFromDeal(t, domain.InitialDeal{
		Hands: [][]domain.Card{
			{attack},
			{defense},
		},
		Stock: []domain.Card{
			{Rank: domain.Eight, Suit: domain.Clubs},
			{Rank: domain.Nine, Suit: domain.Clubs},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Clubs},
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Eight, Suit: domain.Diamonds},
			{Rank: domain.Nine, Suit: domain.Diamonds},
			{Rank: domain.Ten, Suit: domain.Diamonds},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}), recorder)

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	events := recorder.Events()
	expectEventKinds(t, events,
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindAttack,
		domain.EventKindDefend,
		domain.EventKindFinishDefense,
		domain.EventKindRefill,
		domain.EventKindRefill,
		domain.EventKindRoundEnded,
	)
	if events[5].Domain.Refill.Seat != domain.Seat(0) || events[5].Domain.Refill.Drawn != 6 {
		t.Fatalf("first refill = %+v, want seat 0 drawn 6", events[5].Domain.Refill)
	}
	if events[6].Domain.Refill.Seat != domain.Seat(1) || events[6].Domain.Refill.Drawn != 6 {
		t.Fatalf("second refill = %+v, want seat 1 drawn 6", events[6].Domain.Refill)
	}
}

func TestSessionKeepsPendingEventsWhenSinkFails(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	sink := &sequenceFailSink{failSequence: 3}
	session := mustSessionWithSink(t, mustMatch(t, [][]domain.Card{
		{attack},
		{defense},
	}), sink)

	err := session.ApplyAction(context.Background(), domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: attack,
	})
	if !errors.Is(err, errSequenceSinkFailed) {
		t.Fatalf("ApplyAction error = %v, want sink failure", err)
	}

	sink.failSequence = 0
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})

	expectEventKinds(t, sink.events,
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindAttack,
		domain.EventKindDefend,
	)
	if sink.events[2].Sequence != 3 || sink.events[3].Sequence != 4 {
		t.Fatalf("replayed sequences = %d/%d, want 3/4", sink.events[2].Sequence, sink.events[3].Sequence)
	}
}

func TestNewDealtSession(t *testing.T) {
	hands := [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Eight, Suit: domain.Hearts},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Queen, Suit: domain.Hearts},
			{Rank: domain.King, Suit: domain.Spades},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Hearts},
			{Rank: domain.Eight, Suit: domain.Spades},
		},
	}
	deck := deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))

	session, deal, err := app.NewDealtSession(2, domain.DefaultRuleProfile(), domain.DealOptions{
		Shuffler: domain.ShuffleFunc(func(cards []domain.Card) {
			copy(cards, deck)
		}),
	})
	if err != nil {
		t.Fatalf("NewDealtSession returned error: %v", err)
	}

	if session.ViewForSeat(domain.Seat(0)).TrumpSuit != domain.Hearts {
		t.Fatalf("TrumpSuit = %v, want Hearts", session.ViewForSeat(domain.Seat(0)).TrumpSuit)
	}
	if !slices.Equal(deal.Hands[0], hands[0]) {
		t.Fatalf("deal hand = %v, want %v", deal.Hands[0], hands[0])
	}
}

func expectEventKinds(t *testing.T, events []app.Event, kinds ...domain.EventKind) {
	t.Helper()
	if len(events) != len(kinds) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(kinds), events)
	}
	for i, kind := range kinds {
		if events[i].Domain.Kind != kind {
			t.Fatalf("event %d kind = %v, want %v", i, events[i].Domain.Kind, kind)
		}
	}
}

type strategyFunc func(context.Context, *app.DecisionContext) (domain.Action, error)

func (fn strategyFunc) ChooseAction(ctx context.Context, decision *app.DecisionContext) (domain.Action, error) {
	return fn(ctx, decision)
}

func mustSession(t *testing.T, match *domain.Match) *app.Session {
	t.Helper()
	session, err := app.NewSession(match)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	return session
}

func mustSessionWithSink(t *testing.T, match *domain.Match, sink app.EventSink) *app.Session {
	t.Helper()
	session, err := app.NewSessionWithOptions(context.Background(), match, app.SessionOptions{EventSink: sink})
	if err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}
	return session
}

func mustApply(t *testing.T, session *app.Session, action domain.Action) {
	t.Helper()
	if err := session.ApplyAction(context.Background(), action); err != nil {
		t.Fatalf("ApplyAction(%+v) returned error: %v", action, err)
	}
}

var errSequenceSinkFailed = errors.New("sequence sink failed")

type sequenceFailSink struct {
	events       []app.Event
	failSequence uint64
}

func (s *sequenceFailSink) RecordEvent(ctx context.Context, event app.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if event.Sequence == s.failSequence {
		return errSequenceSinkFailed
	}
	s.events = append(s.events, event)
	return nil
}

func mustMatch(t *testing.T, hands [][]domain.Card) *domain.Match {
	t.Helper()
	deal := domain.InitialDeal{
		Hands:          hands,
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}
	return mustMatchFromDeal(t, deal)
}

func mustMatchFromDeal(t *testing.T, deal domain.InitialDeal) *domain.Match {
	t.Helper()
	match, err := domain.NewMatch(&deal, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	return match
}

func deckForDeal(hands [][]domain.Card, stock []domain.Card) []domain.Card {
	deck := make([]domain.Card, 0, 36)
	for cardIndex := range len(hands[0]) {
		for player := range hands {
			deck = append(deck, hands[player][cardIndex])
		}
	}
	deck = append(deck, stock...)
	return deck
}

func stockWithBottom(bottom domain.Card, hands ...[]domain.Card) []domain.Card {
	used := make(map[domain.Card]bool)
	for _, hand := range hands {
		for _, card := range hand {
			used[card] = true
		}
	}
	used[bottom] = true

	stock := make([]domain.Card, 0, 36)
	for _, card := range domain.NewDeck36() {
		if !used[card] {
			stock = append(stock, card)
		}
	}
	return append(stock, bottom)
}
