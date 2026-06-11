package domain_test

import (
	"errors"
	"slices"
	"testing"

	. "github.com/vovakirdan/durak/internal/domain"
)

func TestNewMatchStartsFromInitialDeal(t *testing.T) {
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{{Rank: Six, Suit: Clubs}},
			{{Rank: Seven, Suit: Clubs}},
		},
		[]Card{{Rank: Nine, Suit: Hearts}},
		1,
	))

	if match.Phase() != MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", match.Phase())
	}
	if match.Attacker() != Seat(1) {
		t.Fatalf("Attacker = %d, want 1", match.Attacker())
	}
	if match.Defender() != Seat(0) {
		t.Fatalf("Defender = %d, want 0", match.Defender())
	}
	if match.TrumpSuit() != Hearts {
		t.Fatalf("TrumpSuit = %v, want Hearts", match.TrumpSuit())
	}
}

func TestMatchSnapshotsDoNotExposeMutableSlices(t *testing.T) {
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{{Rank: Six, Suit: Clubs}},
			{{Rank: Seven, Suit: Clubs}},
		},
		[]Card{{Rank: Nine, Suit: Hearts}},
		0,
	))

	hand := match.Hand(Seat(0))
	hand[0] = Card{Rank: Ace, Suit: Spades}
	if got := match.Hand(Seat(0))[0]; got != (Card{Rank: Six, Suit: Clubs}) {
		t.Fatalf("hand mutation leaked into match: %v", got)
	}

	stock := match.Stock()
	stock[0] = Card{Rank: Ace, Suit: Spades}
	if got := match.Stock()[0]; got != (Card{Rank: Nine, Suit: Hearts}) {
		t.Fatalf("stock mutation leaked into match: %v", got)
	}
}

func TestAttackMovesCardToTable(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack},
			{{Rank: Seven, Suit: Clubs}},
		},
		nil,
		0,
	))

	if err := match.Attack(Seat(0), attack); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}

	if match.Phase() != MatchPhaseDefense {
		t.Fatalf("Phase = %v, want MatchPhaseDefense", match.Phase())
	}
	if len(match.Hand(Seat(0))) != 0 {
		t.Fatalf("attacker hand = %v, want empty", match.Hand(Seat(0)))
	}
	if got := match.Table(); len(got) != 1 || got[0].Attack != attack || got[0].Defended {
		t.Fatalf("table = %v, want one undefended attack", got)
	}
}

func TestDefendRequiresBeatingCard(t *testing.T) {
	attack := Card{Rank: King, Suit: Clubs}
	badDefense := Card{Rank: Queen, Suit: Clubs}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack},
			{badDefense},
		},
		nil,
		0,
	))
	if err := match.Attack(Seat(0), attack); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}

	err := match.Defend(Seat(1), 0, badDefense)
	if !errors.Is(err, ErrCardDoesNotBeat) {
		t.Fatalf("Defend error = %v, want ErrCardDoesNotBeat", err)
	}
	if !slices.Contains(match.Hand(Seat(1)), badDefense) {
		t.Fatalf("defense card was removed after failed defend")
	}
}

func TestDefendMovesBeatingCardToTable(t *testing.T) {
	attack := Card{Rank: King, Suit: Clubs}
	defense := Card{Rank: Six, Suit: Hearts}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack},
			{defense},
		},
		nil,
		0,
	))
	if err := match.Attack(Seat(0), attack); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}

	if err := match.Defend(Seat(1), 0, defense); err != nil {
		t.Fatalf("Defend returned error: %v", err)
	}

	if match.Phase() != MatchPhaseThrowIn {
		t.Fatalf("Phase = %v, want MatchPhaseThrowIn", match.Phase())
	}
	if got := match.Table(); len(got) != 1 || !got[0].Defended || got[0].Defense != defense {
		t.Fatalf("table = %v, want defended pair", got)
	}
	if len(match.Hand(Seat(1))) != 0 {
		t.Fatalf("defender hand = %v, want empty", match.Hand(Seat(1)))
	}
}

func TestThrowInRequiresRankOnTable(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	defense := Card{Rank: Seven, Suit: Clubs}
	validThrowIn := Card{Rank: Six, Suit: Diamonds}
	invalidThrowIn := Card{Rank: Ace, Suit: Spades}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack, validThrowIn, invalidThrowIn},
			{defense},
		},
		nil,
		0,
	))
	mustRoundDefended(t, match, attack, defense)

	err := match.ThrowIn(Seat(0), invalidThrowIn)
	if !errors.Is(err, ErrThrowInRankUnavailable) {
		t.Fatalf("ThrowIn error = %v, want ErrThrowInRankUnavailable", err)
	}
	if !slices.Contains(match.Hand(Seat(0)), invalidThrowIn) {
		t.Fatalf("invalid throw-in card was removed")
	}
	if err := match.ThrowIn(Seat(0), validThrowIn); err != nil {
		t.Fatalf("ThrowIn returned error: %v", err)
	}

	if match.Phase() != MatchPhaseDefense {
		t.Fatalf("Phase = %v, want MatchPhaseDefense", match.Phase())
	}
	if got := match.Table(); len(got) != 2 || got[1].Attack != validThrowIn || got[1].Defended {
		t.Fatalf("table = %v, want second undefended attack", got)
	}
}

func TestLegalActionsAndApplyActionUseSameValidationPath(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	throwIn := Card{Rank: Six, Suit: Diamonds}
	defense := Card{Rank: Seven, Suit: Clubs}
	throwInDefense := Card{Rank: Eight, Suit: Diamonds}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack, throwIn},
			{defense, throwInDefense},
		},
		nil,
		0,
	))

	attackAction := Action{Kind: ActionKindAttack, Seat: Seat(0), Card: attack}
	if got := match.LegalActions(Seat(0)); !slices.Contains(got, attackAction) {
		t.Fatalf("LegalActions attacker = %v, want attack action", got)
	}
	if got := match.LegalActions(Seat(1)); len(got) != 0 {
		t.Fatalf("LegalActions defender before attack = %v, want none", got)
	}
	if err := match.ApplyAction(attackAction); err != nil {
		t.Fatalf("ApplyAction attack returned error: %v", err)
	}

	defendAction := Action{Kind: ActionKindDefend, Seat: Seat(1), Card: defense}
	takeAction := Action{Kind: ActionKindTake, Seat: Seat(1)}
	if got := match.LegalActions(Seat(1)); !slices.Contains(got, defendAction) || !slices.Contains(got, takeAction) {
		t.Fatalf("LegalActions defender = %v, want defend and take", got)
	}
	if err := match.ApplyAction(defendAction); err != nil {
		t.Fatalf("ApplyAction defend returned error: %v", err)
	}

	throwInAction := Action{Kind: ActionKindThrowIn, Seat: Seat(0), Card: throwIn}
	finishDefenseAction := Action{Kind: ActionKindFinishDefense, Seat: Seat(0)}
	if got := match.LegalActions(Seat(0)); !slices.Contains(got, throwInAction) || !slices.Contains(got, finishDefenseAction) {
		t.Fatalf("LegalActions throw-in = %v, want throw-in and finish defense", got)
	}
	if err := match.ApplyAction(throwInAction); err != nil {
		t.Fatalf("ApplyAction throw-in returned error: %v", err)
	}

	throwInDefendAction := Action{Kind: ActionKindDefend, Seat: Seat(1), Card: throwInDefense, AttackIndex: 1}
	if got := match.LegalActions(Seat(1)); !slices.Contains(got, throwInDefendAction) {
		t.Fatalf("LegalActions defender after throw-in = %v, want second defense", got)
	}
	if err := match.ApplyAction(Action{}); !errors.Is(err, ErrInvalidAction) {
		t.Fatalf("ApplyAction empty action error = %v, want ErrInvalidAction", err)
	}
}

func TestFirstSuccessfulDefenseAttackLimitStopsSixthAttack(t *testing.T) {
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
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			append(slices.Clone(attacks), extra),
			defenses,
		},
		nil,
		0,
	))

	if err := match.Attack(Seat(0), attacks[0]); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}
	for i, defense := range defenses {
		if err := match.Defend(Seat(1), i, defense); err != nil {
			t.Fatalf("Defend %d returned error: %v", i, err)
		}
		if i == len(defenses)-1 {
			break
		}
		if err := match.ThrowIn(Seat(0), attacks[i+1]); err != nil {
			t.Fatalf("ThrowIn %d returned error: %v", i+1, err)
		}
	}

	err := match.ThrowIn(Seat(0), extra)
	if !errors.Is(err, ErrAttackLimitReached) {
		t.Fatalf("ThrowIn error = %v, want ErrAttackLimitReached", err)
	}
}

func TestFinishDefenseDiscardsRefillsAndSwapsRoles(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	defense := Card{Rank: Seven, Suit: Clubs}
	stock := []Card{
		{Rank: Eight, Suit: Clubs},
		{Rank: Nine, Suit: Clubs},
		{Rank: Ten, Suit: Clubs},
		{Rank: Jack, Suit: Clubs},
		{Rank: Queen, Suit: Clubs},
		{Rank: King, Suit: Clubs},
		{Rank: Six, Suit: Diamonds},
		{Rank: Seven, Suit: Diamonds},
		{Rank: Eight, Suit: Diamonds},
		{Rank: Nine, Suit: Diamonds},
	}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack},
			{defense},
		},
		stock,
		0,
	))
	mustRoundDefended(t, match, attack, defense)

	if err := match.FinishDefense(Seat(0)); err != nil {
		t.Fatalf("FinishDefense returned error: %v", err)
	}

	if match.Phase() != MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", match.Phase())
	}
	if match.Attacker() != Seat(1) || match.Defender() != Seat(0) {
		t.Fatalf("roles = attacker %d defender %d, want 1/0", match.Attacker(), match.Defender())
	}
	if match.SuccessfulDefenses() != 1 {
		t.Fatalf("SuccessfulDefenses = %d, want 1", match.SuccessfulDefenses())
	}
	if got := match.Discard(); !slices.Equal(got, []Card{attack, defense}) {
		t.Fatalf("Discard = %v, want attack and defense", got)
	}
	if got := match.Hand(Seat(0)); len(got) != 6 {
		t.Fatalf("attacker refilled to %d cards, want 6", len(got))
	}
	if got := match.Hand(Seat(1)); len(got) != 4 {
		t.Fatalf("defender refilled to %d cards, want 4 after attacker draws first", len(got))
	}
	if len(match.Stock()) != 0 {
		t.Fatalf("Stock = %v, want empty", match.Stock())
	}
}

func TestTakeMovesTableToDefenderAndKeepsTwoPlayerAttacker(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	defenderCard := Card{Rank: Seven, Suit: Diamonds}
	draw := Card{Rank: Eight, Suit: Clubs}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack},
			{defenderCard},
		},
		[]Card{draw},
		0,
	))
	if err := match.Attack(Seat(0), attack); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}

	if err := match.Take(Seat(1)); err != nil {
		t.Fatalf("Take returned error: %v", err)
	}

	if match.Phase() != MatchPhaseTaking {
		t.Fatalf("Phase = %v, want MatchPhaseTaking", match.Phase())
	}
	if got := match.Hand(Seat(1)); !slices.Equal(got, []Card{defenderCard}) {
		t.Fatalf("defender hand before finish take = %v, want original card", got)
	}
	if err := match.FinishTake(Seat(0)); err != nil {
		t.Fatalf("FinishTake returned error: %v", err)
	}

	if match.Phase() != MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", match.Phase())
	}
	if match.Attacker() != Seat(0) || match.Defender() != Seat(1) {
		t.Fatalf("roles = attacker %d defender %d, want 0/1", match.Attacker(), match.Defender())
	}
	if got := match.Hand(Seat(0)); !slices.Equal(got, []Card{draw}) {
		t.Fatalf("attacker hand = %v, want drawn card", got)
	}
	if got := match.Hand(Seat(1)); !slices.Equal(got, []Card{defenderCard, attack}) {
		t.Fatalf("defender hand = %v, want original card plus taken attack", got)
	}
}

func TestTakingAllowsThrowInsBeyondFirstSuccessfulDefenseLimit(t *testing.T) {
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
	}
	extra := Card{Rank: Six, Suit: Hearts}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			append(slices.Clone(attacks), extra),
			defenses,
		},
		nil,
		0,
	))

	if err := match.Attack(Seat(0), attacks[0]); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}
	for i, defense := range defenses {
		if err := match.Defend(Seat(1), i, defense); err != nil {
			t.Fatalf("Defend %d returned error: %v", i, err)
		}
		if err := match.ThrowIn(Seat(0), attacks[i+1]); err != nil {
			t.Fatalf("ThrowIn %d returned error: %v", i+1, err)
		}
	}
	if err := match.Take(Seat(1)); err != nil {
		t.Fatalf("Take returned error: %v", err)
	}

	if err := match.ThrowIn(Seat(0), extra); err != nil {
		t.Fatalf("ThrowIn after take returned error: %v", err)
	}
	if got := match.Table(); len(got) != 6 {
		t.Fatalf("table has %d attack pairs, want 6", len(got))
	}
	if err := match.FinishTake(Seat(0)); err != nil {
		t.Fatalf("FinishTake returned error: %v", err)
	}
	if got := match.Hand(Seat(1)); len(got) != 10 {
		t.Fatalf("defender took %d cards, want 10 table cards", len(got))
	}
}

func TestMatchCompletesWhenStockIsEmptyAndPlayerRunsOutAfterDefense(t *testing.T) {
	attack := Card{Rank: Six, Suit: Clubs}
	defense := Card{Rank: Seven, Suit: Clubs}
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{attack},
			{defense, {Rank: Ace, Suit: Spades}},
		},
		nil,
		0,
	))
	mustRoundDefended(t, match, attack, defense)

	if err := match.FinishDefense(Seat(0)); err != nil {
		t.Fatalf("FinishDefense returned error: %v", err)
	}

	if match.Phase() != MatchPhaseComplete {
		t.Fatalf("Phase = %v, want MatchPhaseComplete", match.Phase())
	}
	if match.Winner() != Seat(0) || match.Loser() != Seat(1) {
		t.Fatalf("winner/loser = %d/%d, want 0/1", match.Winner(), match.Loser())
	}
	if err := match.Attack(Seat(1), Card{Rank: Ace, Suit: Spades}); !errors.Is(err, ErrMatchComplete) {
		t.Fatalf("Attack after completion error = %v, want ErrMatchComplete", err)
	}
}

func TestConcedeCompletesMatch(t *testing.T) {
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{{Rank: Six, Suit: Clubs}},
			{{Rank: Seven, Suit: Clubs}},
		},
		nil,
		0,
	))

	if err := match.Concede(Seat(1)); err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}

	if match.Phase() != MatchPhaseComplete {
		t.Fatalf("Phase = %v, want MatchPhaseComplete", match.Phase())
	}
	if match.Winner() != Seat(0) || match.Loser() != Seat(1) {
		t.Fatalf("winner/loser = %d/%d, want 0/1", match.Winner(), match.Loser())
	}
	events := match.Events()
	if got := events[len(events)-2].Kind; got != EventKindConcede {
		t.Fatalf("penultimate event = %v, want EventKindConcede", got)
	}
	if got := events[len(events)-2].Concede; got == nil || got.Seat != Seat(1) || got.Winner != Seat(0) {
		t.Fatalf("concede event = %+v, want seat 1 winner 0", got)
	}
	if got := events[len(events)-1].Kind; got != EventKindMatchEnded {
		t.Fatalf("last event = %v, want EventKindMatchEnded", got)
	}
	if err := match.Concede(Seat(1)); !errors.Is(err, ErrMatchComplete) {
		t.Fatalf("Concede after completion error = %v, want ErrMatchComplete", err)
	}
}

func TestConcedeRejectsInvalidSeat(t *testing.T) {
	match := mustNewMatch(t, matchDeal(
		[][]Card{
			{{Rank: Six, Suit: Clubs}},
			{{Rank: Seven, Suit: Clubs}},
		},
		nil,
		0,
	))

	err := match.Concede(Seat(2))
	if !errors.Is(err, ErrInvalidSeat) {
		t.Fatalf("Concede error = %v, want ErrInvalidSeat", err)
	}
	if match.Phase() != MatchPhaseAttack {
		t.Fatalf("Phase = %v, want MatchPhaseAttack", match.Phase())
	}
}

func TestNewMatchRejectsMoreThanTwoPlayersUntilMultiSeatFlowExists(t *testing.T) {
	deal := matchDeal(
		[][]Card{
			{{Rank: Six, Suit: Clubs}},
			{{Rank: Seven, Suit: Clubs}},
			{{Rank: Eight, Suit: Clubs}},
		},
		nil,
		0,
	)
	_, err := NewMatch(&deal, DefaultRuleProfile())
	if !errors.Is(err, ErrInvalidPlayerCount) {
		t.Fatalf("NewMatch error = %v, want ErrInvalidPlayerCount", err)
	}
}

func mustRoundDefended(t *testing.T, match *Match, attack, defense Card) {
	t.Helper()
	if err := match.Attack(match.Attacker(), attack); err != nil {
		t.Fatalf("Attack returned error: %v", err)
	}
	if err := match.Defend(match.Defender(), 0, defense); err != nil {
		t.Fatalf("Defend returned error: %v", err)
	}
}

func mustNewMatch(t *testing.T, deal InitialDeal) *Match {
	t.Helper()
	match, err := NewMatch(&deal, DefaultRuleProfile())
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	return match
}

func matchDeal(hands [][]Card, stock []Card, firstAttacker int) InitialDeal {
	return InitialDeal{
		Hands:          hands,
		Stock:          stock,
		TrumpIndicator: Card{Rank: Nine, Suit: Hearts},
		TrumpSuit:      Hearts,
		FirstAttacker:  firstAttacker,
	}
}
