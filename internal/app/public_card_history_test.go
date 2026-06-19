package app_test

import (
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestPublicCardMemoryOverlaysOwnHand(t *testing.T) {
	own := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	opponent := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	session := mustSession(t, mustMatch(t, [][]domain.Card{{own}, {opponent}}))

	memory := session.DecisionContext(domain.Seat(0)).PublicMemory

	if !slices.Equal(memory.Hand, []domain.Card{own}) {
		t.Fatalf("Hand = %v, want own hand", memory.Hand)
	}
	if !slices.Equal(memory.KnownHeld[0], []domain.Card{own}) {
		t.Fatalf("KnownHeld[0] = %v, want own hand", memory.KnownHeld[0])
	}
	if len(memory.KnownHeld[1]) != 0 {
		t.Fatalf("KnownHeld[1] = %v, want hidden opponent hand", memory.KnownHeld[1])
	}
	if !slices.Contains(memory.UnknownPool, opponent) {
		t.Fatalf("%v not in unknown pool, want hidden opponent card unknown", opponent)
	}
	if slices.Contains(memory.UnknownPool, own) {
		t.Fatalf("%v in unknown pool, want own card known", own)
	}
	if slices.Contains(memory.UnknownPool, memory.TrumpIndicator) {
		t.Fatalf("%v in unknown pool, want visible trump known", memory.TrumpIndicator)
	}
}

func TestPublicCardMemoryTracksCardsTakenByDefender(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	attackerExtra := domain.Card{Rank: domain.Eight, Suit: domain.Diamonds}
	defender := domain.Card{Rank: domain.Seven, Suit: domain.Hearts}
	session := mustSession(t, mustMatch(t, [][]domain.Card{
		{attack, attackerExtra},
		{defender},
	}))

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindTake, Seat: domain.Seat(1)})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishTake, Seat: domain.Seat(0)})

	memory := session.DecisionContext(domain.Seat(0)).PublicMemory
	if !slices.Contains(memory.KnownHeld[1], attack) {
		t.Fatalf("KnownHeld[1] = %v, want taken attack card %v", memory.KnownHeld[1], attack)
	}
	if slices.Contains(memory.UnknownPool, attack) {
		t.Fatalf("%v in unknown pool, want taken card known", attack)
	}
	if slices.Contains(memory.Discard, attack) {
		t.Fatalf("Discard = %v, want taken card not discarded", memory.Discard)
	}
}

func TestPublicCardMemoryDoesNotRevealOpponentRefillCards(t *testing.T) {
	attack := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	defense := domain.Card{Rank: domain.Seven, Suit: domain.Clubs}
	opponentDraw := domain.Card{Rank: domain.Six, Suit: domain.Diamonds}
	stock := []domain.Card{
		{Rank: domain.Eight, Suit: domain.Clubs},
		{Rank: domain.Nine, Suit: domain.Clubs},
		{Rank: domain.Ten, Suit: domain.Clubs},
		{Rank: domain.Jack, Suit: domain.Clubs},
		{Rank: domain.Queen, Suit: domain.Clubs},
		{Rank: domain.King, Suit: domain.Clubs},
		opponentDraw,
		{Rank: domain.Seven, Suit: domain.Diamonds},
		{Rank: domain.Eight, Suit: domain.Diamonds},
		{Rank: domain.Nine, Suit: domain.Diamonds},
		{Rank: domain.Ten, Suit: domain.Diamonds},
		{Rank: domain.Jack, Suit: domain.Diamonds},
	}
	session := mustSession(t, mustMatchFromDeal(t, domain.InitialDeal{
		Hands: [][]domain.Card{
			{attack},
			{defense},
		},
		Stock:          stock,
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	}))

	mustApply(t, session, domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: attack})
	mustApply(t, session, domain.Action{
		Kind:        domain.ActionKindDefend,
		Seat:        domain.Seat(1),
		Card:        defense,
		AttackIndex: 0,
	})
	mustApply(t, session, domain.Action{Kind: domain.ActionKindFinishDefense, Seat: domain.Seat(0)})

	memory := session.DecisionContext(domain.Seat(0)).PublicMemory
	if !slices.Contains(memory.Discard, attack) || !slices.Contains(memory.Discard, defense) {
		t.Fatalf("Discard = %v, want defended table cards", memory.Discard)
	}
	if slices.Contains(memory.KnownHeld[1], opponentDraw) {
		t.Fatalf("KnownHeld[1] = %v, want refill card hidden", memory.KnownHeld[1])
	}
	if !slices.Contains(memory.UnknownPool, opponentDraw) {
		t.Fatalf("%v not in unknown pool, want opponent refill card unknown", opponentDraw)
	}
}

func TestPublicCardMemoryRemovesKnownTakenCardWhenPlayed(t *testing.T) {
	taken := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	secondTaken := domain.Card{Rank: domain.Six, Suit: domain.Diamonds}
	history := app.NewPublicCardHistory()
	history.Apply(domain.Event{
		Kind: domain.EventKindMatchStarted,
		Started: &domain.MatchStartedEvent{
			PlayerCount: 2,
		},
	})
	history.Apply(domain.Event{
		Kind: domain.EventKindDeal,
		Deal: &domain.DealEvent{
			TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
			TrumpSuit:      domain.Hearts,
			HandSizes:      []int{1, 1},
			StockCount:     22,
		},
	})
	history.Apply(domain.Event{
		Kind: domain.EventKindRoundEnded,
		RoundEnded: &domain.RoundEndedEvent{
			Outcome:  domain.RoundOutcomeTake,
			Defender: domain.Seat(1),
			Cards:    []domain.Card{taken, secondTaken},
		},
	})

	before := history.Snapshot(domain.Seat(0), nil)
	if !slices.Contains(before.KnownHeld[1], taken) || !slices.Contains(before.KnownHeld[1], secondTaken) {
		t.Fatalf("KnownHeld[1] = %v, want taken card before play", before.KnownHeld[1])
	}

	history.Apply(domain.Event{
		Kind: domain.EventKindAttack,
		Action: &domain.ActionEvent{
			Action: domain.NewAttackAction(domain.Seat(1), taken, secondTaken),
		},
	})

	after := history.Snapshot(domain.Seat(0), nil)
	if slices.Contains(after.KnownHeld[1], taken) || slices.Contains(after.KnownHeld[1], secondTaken) {
		t.Fatalf("KnownHeld[1] = %v, want played known card removed", after.KnownHeld[1])
	}
	if !slices.Contains(after.Seen, taken) || !slices.Contains(after.Seen, secondTaken) {
		t.Fatalf("Seen = %v, want played packet cards still known", after.Seen)
	}
}

var _ = app.PublicCardMemory{}
