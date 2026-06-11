package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRunWithOptionsStartsAndQuits(t *testing.T) {
	var out bytes.Buffer

	err := RunWithOptions(context.Background(), strings.NewReader("q\n"), &out, &RunOptions{
		Strategy: firstLegalStrategy(),
	})
	if err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Durak CLI") {
		t.Fatalf("output = %q, want header", output)
	}
	if !strings.Contains(output, "Bye.") {
		t.Fatalf("output = %q, want quit message", output)
	}
}

func TestRunWithOptionsUsesDeterministicDeal(t *testing.T) {
	hands := [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Hearts},
			{Rank: domain.Eight, Suit: domain.Diamonds},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.Ten, Suit: domain.Hearts},
			{Rank: domain.Ace, Suit: domain.Clubs},
			{Rank: domain.Six, Suit: domain.Diamonds},
			{Rank: domain.Seven, Suit: domain.Spades},
			{Rank: domain.Eight, Suit: domain.Spades},
		},
	}
	deck := testDeckForDeal(hands, testStockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))
	var out bytes.Buffer

	err := RunWithOptions(context.Background(), strings.NewReader("q\n"), &out, &RunOptions{
		Deal: domain.DealOptions{
			Shuffler: domain.ShuffleFunc(func(cards []domain.Card) {
				copy(cards, deck)
			}),
		},
		Strategy: firstLegalStrategy(),
	})
	if err != nil {
		t.Fatalf("RunWithOptions returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Trump: H (9H)") {
		t.Fatalf("output = %q, want deterministic trump", output)
	}
	if !strings.Contains(output, "Attacker: you(0)") {
		t.Fatalf("output = %q, want human attacker from deterministic deal", output)
	}
	if !strings.Contains(output, "Hand: 1:6C 2:7H") {
		t.Fatalf("output = %q, want deterministic hand order", output)
	}
}

func TestGameCompletesScriptedRound(t *testing.T) {
	session := mustCLISession(t, domain.InitialDeal{
		Hands: [][]domain.Card{
			{{Rank: domain.Six, Suit: domain.Clubs}},
			{
				{Rank: domain.Seven, Suit: domain.Clubs},
				{Rank: domain.Ace, Suit: domain.Spades},
			},
		},
		TrumpIndicator: domain.Card{Rank: domain.Nine, Suit: domain.Hearts},
		TrumpSuit:      domain.Hearts,
		FirstAttacker:  0,
	})
	var out bytes.Buffer
	game := newGame(session, firstLegalStrategy(), strings.NewReader("1\ndone\n"), &out, gameOptions{
		humanSeat: defaultHumanSeat,
		botSeat:   defaultBotSeat,
	})

	if err := game.run(context.Background()); err != nil {
		t.Fatalf("game run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Bot: defend 1 with 7C") {
		t.Fatalf("output = %q, want bot defense", output)
	}
	if !strings.Contains(output, "Result: you won") {
		t.Fatalf("output = %q, want human win", output)
	}
}

func TestRunWithOptionsRejectsMissingStrategy(t *testing.T) {
	var out bytes.Buffer

	err := RunWithOptions(context.Background(), strings.NewReader("q\n"), &out, &RunOptions{})
	if err == nil {
		t.Fatal("RunWithOptions returned nil error, want missing strategy")
	}
}

type strategyFunc func(context.Context, *app.DecisionContext) (domain.Action, error)

func (fn strategyFunc) ChooseAction(ctx context.Context, decision *app.DecisionContext) (domain.Action, error) {
	return fn(ctx, decision)
}

func firstLegalStrategy() strategyFunc {
	return func(ctx context.Context, decision *app.DecisionContext) (domain.Action, error) {
		if err := ctx.Err(); err != nil {
			return domain.Action{}, err
		}
		if decision == nil || len(decision.LegalActions) == 0 {
			return domain.Action{}, commandError("no legal action")
		}
		return decision.LegalActions[0], nil
	}
}

func mustCLISession(t *testing.T, deal domain.InitialDeal) *app.Session {
	t.Helper()
	match, err := domain.NewMatch(&deal, domain.DefaultRuleProfile())
	if err != nil {
		t.Fatalf("NewMatch returned error: %v", err)
	}
	session, err := app.NewSession(match)
	if err != nil {
		t.Fatalf("NewSession returned error: %v", err)
	}
	return session
}

func testDeckForDeal(hands [][]domain.Card, stock []domain.Card) []domain.Card {
	deck := make([]domain.Card, 0, 36)
	for cardIndex := range len(hands[0]) {
		for player := range hands {
			deck = append(deck, hands[player][cardIndex])
		}
	}
	return append(deck, stock...)
}

func testStockWithBottom(bottom domain.Card, hands ...[]domain.Card) []domain.Card {
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
