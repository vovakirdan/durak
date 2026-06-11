package domain_test

import (
	"errors"
	"slices"
	"testing"

	. "github.com/vovakirdan/durak/internal/domain"
)

func TestDefaultRuleScenarioDrawAfterBothHandsEmpty(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	defense := Card{Rank: Seven, Suit: Clubs}
	scenario := newMatchScenario(t, [][]Card{
		{attack},
		{defense},
	})

	scenario.attack(Seat(0), attack)
	scenario.defend(Seat(1), 0, defense)
	scenario.finishDefense(Seat(0))

	scenario.expectPhase(MatchPhaseComplete)
	scenario.expectWinner(NoSeat, NoSeat)
	scenario.expectHand(Seat(0), nil)
	scenario.expectHand(Seat(1), nil)
}

func TestDefaultRuleScenarioWinnerLoserAfterDefense(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	defense := Card{Rank: Seven, Suit: Clubs}
	extra := Card{Rank: Ace, Suit: Spades}
	scenario := newMatchScenario(t, [][]Card{
		{attack},
		{defense, extra},
	})

	scenario.attack(Seat(0), attack)
	scenario.defend(Seat(1), 0, defense)
	scenario.finishDefense(Seat(0))

	scenario.expectPhase(MatchPhaseComplete)
	scenario.expectWinner(Seat(0), Seat(1))
	scenario.expectHand(Seat(1), []Card{extra})
}

func TestDefaultRuleScenarioDefenderTakesAllThrowIns(t *testing.T) {
	attacks := []Card{
		{Rank: Six, Suit: Clubs},
		{Rank: Six, Suit: Diamonds},
		{Rank: Six, Suit: Spades},
		{Rank: Six, Suit: Hearts},
	}
	extraAttackerCard := Card{Rank: Ace, Suit: Spades}
	defenderCard := Card{Rank: Nine, Suit: Diamonds}
	scenario := newMatchScenario(t, [][]Card{
		append(slices.Clone(attacks), extraAttackerCard),
		{defenderCard},
	})

	scenario.attack(Seat(0), attacks[0])
	scenario.take(Seat(1))
	for _, card := range attacks[1:] {
		scenario.throwIn(Seat(0), card)
	}
	scenario.finishTake(Seat(0))

	scenario.expectPhase(MatchPhaseAttack)
	scenario.expectRoles(Seat(0), Seat(1))
	scenario.expectHand(Seat(0), []Card{extraAttackerCard})
	scenario.expectHand(Seat(1), []Card{
		defenderCard,
		attacks[0],
		attacks[1],
		attacks[2],
		attacks[3],
	})
}

func TestDefaultRuleScenarioFirstAttackCannotBeTransferred(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	transfer := Card{Rank: Six, Suit: Diamonds}
	scenario := newMatchScenario(t, [][]Card{
		{attack},
		{transfer},
	})

	scenario.attack(Seat(0), attack)

	action := Action{Kind: ActionKindTransfer, Seat: Seat(1), Card: transfer}
	scenario.expectNotLegal(action)
	scenario.reject(action, ErrTransferNotAllowed)
}

func TestRuleProfileCanAllowFirstAttackTransfer(t *testing.T) {
	profile := DefaultRuleProfile()
	profile.FirstAttackTransferAllowed = true
	attack := Card{Rank: Six, Suit: Clubs}
	transfer := Card{Rank: Six, Suit: Diamonds}
	scenario := newMatchScenarioWithProfile(t, [][]Card{
		{attack},
		{transfer},
	}, nil, profile)

	scenario.attack(Seat(0), attack)
	scenario.transfer(Seat(1), transfer)

	scenario.expectRoles(Seat(1), Seat(0))
	scenario.expectTable([]TablePair{
		{Attack: attack},
		{Attack: transfer},
	})
}

func TestRuleProfileCanDisableTransfers(t *testing.T) {
	profile := DefaultRuleProfile()
	profile.TransferEnabled = false
	profile.FirstAttackTransferAllowed = true
	attack := Card{Rank: Six, Suit: Clubs}
	transfer := Card{Rank: Six, Suit: Diamonds}
	scenario := newMatchScenarioWithProfile(t, [][]Card{
		{attack},
		{transfer},
	}, nil, profile)

	scenario.attack(Seat(0), attack)

	action := Action{Kind: ActionKindTransfer, Seat: Seat(1), Card: transfer}
	scenario.expectNotLegal(action)
	scenario.reject(action, ErrTransferDisabled)
}

func TestDefaultRuleScenarioTransferAfterFirstAttackRound(t *testing.T) {
	firstAttack := Card{Rank: Six, Suit: Clubs}
	secondAttack := Card{Rank: Seven, Suit: Clubs}
	transfer := Card{Rank: Seven, Suit: Diamonds}
	firstDefense := Card{Rank: Eight, Suit: Clubs}
	secondDefense := Card{Rank: Eight, Suit: Diamonds}
	scenario := newMatchScenario(t, [][]Card{
		{firstAttack, secondAttack, firstDefense, secondDefense},
		{{Rank: Nine, Suit: Spades}, transfer},
	})

	scenario.attack(Seat(0), firstAttack)
	scenario.take(Seat(1))
	scenario.finishTake(Seat(0))
	scenario.attack(Seat(0), secondAttack)
	scenario.transfer(Seat(1), transfer)

	scenario.expectRoles(Seat(1), Seat(0))
	scenario.expectTable([]TablePair{
		{Attack: secondAttack},
		{Attack: transfer},
	})

	scenario.defend(Seat(0), 0, firstDefense)
	scenario.defend(Seat(0), 1, secondDefense)
	scenario.finishDefense(Seat(1))
	scenario.expectWinner(Seat(0), Seat(1))
}

func TestDefaultRuleScenarioFirstSuccessfulDefenseAllowsOnlyFiveAttacks(t *testing.T) {
	attacks := []Card{
		{Rank: Six, Suit: Clubs},
		{Rank: Six, Suit: Diamonds},
		{Rank: Seven, Suit: Spades},
		{Rank: Six, Suit: Spades},
		{Rank: Eight, Suit: Clubs},
	}
	defenses := []Card{
		{Rank: Seven, Suit: Clubs},
		{Rank: Eight, Suit: Diamonds},
		{Rank: Eight, Suit: Spades},
		{Rank: Nine, Suit: Spades},
		{Rank: Nine, Suit: Clubs},
	}
	extra := Card{Rank: Six, Suit: Hearts}
	scenario := newMatchScenario(t, [][]Card{
		append(slices.Clone(attacks), extra),
		defenses,
	})

	scenario.attack(Seat(0), attacks[0])
	for i, defense := range defenses {
		scenario.defend(Seat(1), i, defense)
		if i+1 < len(attacks) {
			scenario.throwIn(Seat(0), attacks[i+1])
		}
	}

	extraThrowIn := Action{Kind: ActionKindThrowIn, Seat: Seat(0), Card: extra}
	scenario.expectNotLegal(extraThrowIn)
	scenario.reject(extraThrowIn, ErrAttackLimitReached)
	scenario.finishDefense(Seat(0))
	scenario.expectSuccessfulDefenses(1)
}

func TestDefaultRuleScenarioLaterSuccessfulDefenseHasNoFiveAttackLimit(t *testing.T) {
	firstAttack := Card{Rank: Six, Suit: Clubs}
	firstDefense := Card{Rank: Seven, Suit: Clubs}
	attacks := []Card{
		{Rank: Six, Suit: Diamonds},
		{Rank: Six, Suit: Spades},
		{Rank: Seven, Suit: Hearts},
		{Rank: Eight, Suit: Diamonds},
		{Rank: Nine, Suit: Clubs},
		{Rank: Six, Suit: Hearts},
	}
	defenses := []Card{
		{Rank: Seven, Suit: Diamonds},
		{Rank: Seven, Suit: Spades},
		{Rank: Eight, Suit: Hearts},
		{Rank: Nine, Suit: Diamonds},
		{Rank: Ten, Suit: Clubs},
		{Rank: Nine, Suit: Hearts},
	}
	scenario := newMatchScenario(t, [][]Card{
		append([]Card{firstAttack}, defenses...),
		append([]Card{firstDefense}, attacks...),
	})

	scenario.attack(Seat(0), firstAttack)
	scenario.defend(Seat(1), 0, firstDefense)
	scenario.finishDefense(Seat(0))
	scenario.expectSuccessfulDefenses(1)
	scenario.expectRoles(Seat(1), Seat(0))

	scenario.attack(Seat(1), attacks[0])
	for i, defense := range defenses {
		scenario.defend(Seat(0), i, defense)
		if i+1 < len(attacks) {
			scenario.throwIn(Seat(1), attacks[i+1])
		}
	}
	scenario.finishDefense(Seat(1))

	scenario.expectSuccessfulDefenses(2)
	scenario.expectPhase(MatchPhaseComplete)
	scenario.expectWinner(NoSeat, NoSeat)
}

type matchScenario struct {
	t     *testing.T
	match *Match
}

func newMatchScenario(t *testing.T, hands [][]Card) *matchScenario {
	t.Helper()
	return newMatchScenarioWithStock(t, hands, nil)
}

func newMatchScenarioWithStock(t *testing.T, hands [][]Card, stock []Card) *matchScenario {
	t.Helper()
	return newMatchScenarioWithProfile(t, hands, stock, DefaultRuleProfile())
}

func newMatchScenarioWithProfile(t *testing.T, hands [][]Card, stock []Card, profile RuleProfile) *matchScenario {
	t.Helper()
	deal := matchDeal(hands, stock, 0)
	match, err := NewMatch(&deal, profile)
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	return &matchScenario{
		t:     t,
		match: match,
	}
}

func (s *matchScenario) attack(seat Seat, card Card) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindAttack, Seat: seat, Card: card})
}

func (s *matchScenario) defend(seat Seat, attackIndex int, card Card) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindDefend, Seat: seat, AttackIndex: attackIndex, Card: card})
}

func (s *matchScenario) throwIn(seat Seat, card Card) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindThrowIn, Seat: seat, Card: card})
}

func (s *matchScenario) transfer(seat Seat, card Card) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindTransfer, Seat: seat, Card: card})
}

func (s *matchScenario) take(seat Seat) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindTake, Seat: seat})
}

func (s *matchScenario) finishDefense(seat Seat) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindFinishDefense, Seat: seat})
}

func (s *matchScenario) finishTake(seat Seat) {
	s.t.Helper()
	s.apply(Action{Kind: ActionKindFinishTake, Seat: seat})
}

func (s *matchScenario) apply(action Action) {
	s.t.Helper()
	if !slices.Contains(s.match.LegalActions(action.Seat), action) {
		s.t.Fatalf("action %+v is not legal; legal actions: %+v", action, s.match.LegalActions(action.Seat))
	}
	if err := s.match.ApplyAction(action); err != nil {
		s.t.Fatalf("ApplyAction(%+v) returned error: %v", action, err)
	}
}

func (s *matchScenario) reject(action Action, want error) {
	s.t.Helper()
	err := s.match.ApplyAction(action)
	if !errors.Is(err, want) {
		s.t.Fatalf("ApplyAction(%+v) error = %v, want %v", action, err, want)
	}
}

func (s *matchScenario) expectNotLegal(action Action) {
	s.t.Helper()
	if slices.Contains(s.match.LegalActions(action.Seat), action) {
		s.t.Fatalf("action %+v is legal, want it rejected", action)
	}
}

func (s *matchScenario) expectPhase(want MatchPhase) {
	s.t.Helper()
	if got := s.match.Phase(); got != want {
		s.t.Fatalf("Phase = %v, want %v", got, want)
	}
}

func (s *matchScenario) expectRoles(attacker, defender Seat) {
	s.t.Helper()
	if s.match.Attacker() != attacker || s.match.Defender() != defender {
		s.t.Fatalf("roles = attacker %d defender %d, want %d/%d",
			s.match.Attacker(), s.match.Defender(), attacker, defender)
	}
}

func (s *matchScenario) expectWinner(winner, loser Seat) {
	s.t.Helper()
	if s.match.Winner() != winner || s.match.Loser() != loser {
		s.t.Fatalf("winner/loser = %d/%d, want %d/%d", s.match.Winner(), s.match.Loser(), winner, loser)
	}
}

func (s *matchScenario) expectSuccessfulDefenses(want int) {
	s.t.Helper()
	if got := s.match.SuccessfulDefenses(); got != want {
		s.t.Fatalf("SuccessfulDefenses = %d, want %d", got, want)
	}
}

func (s *matchScenario) expectHand(seat Seat, want []Card) {
	s.t.Helper()
	if got := s.match.Hand(seat); !slices.Equal(got, want) {
		s.t.Fatalf("Hand(%d) = %v, want %v", seat, got, want)
	}
}

func (s *matchScenario) expectTable(want []TablePair) {
	s.t.Helper()
	if got := s.match.Table(); !slices.Equal(got, want) {
		s.t.Fatalf("Table = %v, want %v", got, want)
	}
}
