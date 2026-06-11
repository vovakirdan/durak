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

func TestSessionApplyPlayerDecisionRejectsNilActionTurn(t *testing.T) {
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))

	err := session.ApplyPlayerDecision(
		context.Background(),
		domain.Seat(0),
		nil,
		app.ActionDecision(domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0)}),
	)
	if !errors.Is(err, app.ErrNilTurn) {
		t.Fatalf("ApplyPlayerDecision error = %v, want ErrNilTurn", err)
	}
}

func TestSessionApplyPlayerDecisionRejectsUnavailableConcede(t *testing.T) {
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}))

	err := session.ApplyPlayerDecision(
		context.Background(),
		domain.Seat(0),
		&app.TurnContext{CanConcede: false},
		app.ConcedeDecision(),
	)
	if !errors.Is(err, app.ErrInvalidPlayerDecision) {
		t.Fatalf("ApplyPlayerDecision error = %v, want ErrInvalidPlayerDecision", err)
	}
	if got := session.ViewForSeat(domain.Seat(0)).Phase; got != domain.MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", got)
	}
}

func TestSessionWithEventStoreEmitsInitialEvents(t *testing.T) {
	store := app.NewInMemoryEventStore()
	session, err := app.NewSessionWithOptions(context.Background(), mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}), app.SessionOptions{MatchID: testMatchID, EventStore: store})
	if err != nil {
		t.Fatalf("NewSessionWithOptions returned error: %v", err)
	}

	if session.ViewForSeat(domain.Seat(0)).Phase != domain.MatchPhaseAttack {
		t.Fatalf("session was not created")
	}
	events := store.Events()
	expectEventKinds(t, events, domain.EventKindMatchStarted, domain.EventKindDeal)
	expectTestMatchID(t, events)
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("sequences = %d/%d, want 1/2", events[0].Sequence, events[1].Sequence)
	}
	if got := events[1].Domain.Deal.HandSizes; !slices.Equal(got, []int{1, 1}) {
		t.Fatalf("deal hand sizes = %v, want [1 1]", got)
	}
}

func TestSessionWithEventStoreRequiresMatchID(t *testing.T) {
	store := app.NewInMemoryEventStore()
	_, err := app.NewSessionWithOptions(context.Background(), mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}), app.SessionOptions{EventStore: store})
	if !errors.Is(err, app.ErrEmptyMatchID) {
		t.Fatalf("NewSessionWithOptions error = %v, want ErrEmptyMatchID", err)
	}
}

func TestSessionWithInternalEventStoreRequiresMatchID(t *testing.T) {
	store := app.NewInMemoryInternalEventStore()
	deal := testInitialDeal()
	_, err := app.NewSessionWithOptions(context.Background(), mustMatchFromDeal(t, deal), app.SessionOptions{
		InternalEventStore: store,
		InitialDeal:        &deal,
	})
	if !errors.Is(err, app.ErrEmptyMatchID) {
		t.Fatalf("NewSessionWithOptions error = %v, want ErrEmptyMatchID", err)
	}
}

func TestSessionWithInternalEventStoreRequiresInitialDeal(t *testing.T) {
	store := app.NewInMemoryInternalEventStore()
	_, err := app.NewSessionWithOptions(context.Background(), mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}), app.SessionOptions{MatchID: testMatchID, InternalEventStore: store})
	if !errors.Is(err, app.ErrMissingInitialDeal) {
		t.Fatalf("NewSessionWithOptions error = %v, want ErrMissingInitialDeal", err)
	}
}

func TestSessionApplyActionEmitsSequencedEvents(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	store := app.NewInMemoryEventStore()
	session := mustSessionWithStore(t, mustMatch(t, [][]domain.Card{
		{attack},
		{defense},
	}), store)

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	events := store.Events()
	expectEventKinds(t, events,
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindAttack,
		domain.EventKindDefend,
		domain.EventKindFinishDefense,
		domain.EventKindRoundEnded,
		domain.EventKindMatchEnded,
	)
	expectTestMatchID(t, events)
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

func TestSessionConcedeEmitsSequencedEvents(t *testing.T) {
	store := app.NewInMemoryEventStore()
	session := mustSessionWithStore(t, mustMatch(t, [][]domain.Card{
		{{Rank: domain.Six, Suit: domain.Clubs}},
		{{Rank: domain.Seven, Suit: domain.Clubs}},
	}), store)

	if err := session.Concede(context.Background(), domain.Seat(0)); err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}

	events := store.Events()
	expectEventKinds(t, events,
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindConcede,
		domain.EventKindMatchEnded,
	)
	expectTestMatchID(t, events)
	if events[2].Sequence != 3 || events[3].Sequence != 4 {
		t.Fatalf("concede sequences = %d/%d, want 3/4", events[2].Sequence, events[3].Sequence)
	}
	if got := events[2].Domain.Concede; got == nil || got.Seat != domain.Seat(0) || got.Winner != domain.Seat(1) {
		t.Fatalf("concede event = %+v, want seat 0 winner 1", got)
	}
	view := session.ViewForSeat(domain.Seat(0))
	if view.Phase != domain.MatchPhaseComplete || view.Winner != domain.Seat(1) || view.Loser != domain.Seat(0) {
		t.Fatalf("view = %+v, want complete winner 1 loser 0", view)
	}
}

func TestSessionEmitsRefillEvents(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	store := app.NewInMemoryEventStore()
	session := mustSessionWithStore(t, mustMatchFromDeal(t, domain.InitialDeal{
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
	}), store)

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	events := store.Events()
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
	expectTestMatchID(t, events)
	if events[5].Domain.Refill.Seat != domain.Seat(0) || events[5].Domain.Refill.Drawn != 6 {
		t.Fatalf("first refill = %+v, want seat 0 drawn 6", events[5].Domain.Refill)
	}
	if events[6].Domain.Refill.Seat != domain.Seat(1) || events[6].Domain.Refill.Drawn != 6 {
		t.Fatalf("second refill = %+v, want seat 1 drawn 6", events[6].Domain.Refill)
	}
}

func TestSessionKeepsPendingEventsWhenStoreFails(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	store := &sequenceFailStore{failSequence: 3}
	session := mustSessionWithStore(t, mustMatch(t, [][]domain.Card{
		{attack},
		{defense},
	}), store)

	err := session.ApplyAction(context.Background(), domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: attack,
	})
	if !errors.Is(err, errSequenceStoreFailed) {
		t.Fatalf("ApplyAction error = %v, want store failure", err)
	}

	store.failSequence = 0
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})

	expectEventKinds(t, store.events,
		domain.EventKindMatchStarted,
		domain.EventKindDeal,
		domain.EventKindAttack,
		domain.EventKindDefend,
	)
	expectTestMatchID(t, store.events)
	if store.events[2].Sequence != 3 || store.events[3].Sequence != 4 {
		t.Fatalf("replayed sequences = %d/%d, want 3/4", store.events[2].Sequence, store.events[3].Sequence)
	}
}

func TestNewDealtSessionWithInternalEventStoreEmitsCanonicalDeal(t *testing.T) {
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
	stock := stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...)
	deck := deckForDeal(hands, stock)
	store := app.NewInMemoryInternalEventStore()

	_, _, err := app.NewDealtSessionWithOptions(context.Background(), 2, domain.DefaultRuleProfile(), domain.DealOptions{
		Shuffler: domain.ShuffleFunc(func(cards []domain.Card) {
			copy(cards, deck)
		}),
	}, app.SessionOptions{MatchID: testMatchID, InternalEventStore: store})
	if err != nil {
		t.Fatalf("NewDealtSessionWithOptions returned error: %v", err)
	}

	events := store.Events()
	if len(events) != 2 {
		t.Fatalf("got %d internal events, want 2: %+v", len(events), events)
	}
	if events[0].Domain.Kind != domain.EventKindMatchStarted || events[1].Domain.Kind != domain.EventKindDeal {
		t.Fatalf("internal event kinds = %v/%v, want started/deal", events[0].Domain.Kind, events[1].Domain.Kind)
	}
	if events[0].Sequence != 1 || events[1].Sequence != 2 {
		t.Fatalf("internal sequences = %d/%d, want 1/2", events[0].Sequence, events[1].Sequence)
	}
	if events[1].Deal == nil {
		t.Fatal("internal deal is nil")
	}
	if !slices.Equal(events[1].Deal.Hands[0], hands[0]) || !slices.Equal(events[1].Deal.Hands[1], hands[1]) {
		t.Fatalf("internal hands = %v, want %v", events[1].Deal.Hands, hands)
	}
	if !slices.Equal(events[1].Deal.Stock, stock) {
		t.Fatalf("internal stock = %v, want %v", events[1].Deal.Stock, stock)
	}
	if got := events[1].Domain.Deal.HandSizes; !slices.Equal(got, []int{6, 6}) {
		t.Fatalf("public deal hand sizes = %v, want [6 6]", got)
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

func expectTestMatchID(t *testing.T, events []app.Event) {
	t.Helper()
	for i, event := range events {
		if event.MatchID != testMatchID {
			t.Fatalf("event %d match id = %q, want %q", i, event.MatchID, testMatchID)
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

func mustSessionWithStore(t *testing.T, match *domain.Match, store app.EventStore) *app.Session {
	t.Helper()
	session, err := app.NewSessionWithOptions(context.Background(), match, app.SessionOptions{
		MatchID:    testMatchID,
		EventStore: store,
	})
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

const testMatchID app.MatchID = "test-match"

var errSequenceStoreFailed = errors.New("sequence store failed")

type sequenceFailStore struct {
	events       []app.Event
	failSequence uint64
}

func (s *sequenceFailStore) AppendEvents(ctx context.Context, events []app.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, event := range events {
		if event.Sequence == s.failSequence {
			return errSequenceStoreFailed
		}
	}
	s.events = append(s.events, events...)
	return nil
}

func mustMatch(t *testing.T, hands [][]domain.Card) *domain.Match {
	t.Helper()
	deal := testInitialDeal()
	deal.Hands = hands
	return mustMatchFromDeal(t, deal)
}

func testInitialDeal() domain.InitialDeal {
	return domain.InitialDeal{
		Hands: [][]domain.Card{
			{{Rank: domain.Six, Suit: domain.Clubs}},
			{{Rank: domain.Seven, Suit: domain.Clubs}},
		},
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}
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
