package client

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestLocalGameSubmitActionUsesCurrentLegalActionID(t *testing.T) {
	game := mustLocalGame(t, domain.Seat(0), firstLegalControllers(domain.Seat(1)))
	state := game.State()
	if state.MatchID != "match-1" || state.Phase != "attack" || len(state.LegalActions) == 0 {
		t.Fatalf("initial state = %+v, want human attack actions", state)
	}

	next, err := game.SubmitAction(context.Background(), "1")
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}

	if next.Version != state.Version+1 {
		t.Fatalf("version = %d, want %d", next.Version, state.Version+1)
	}
	if next.Phase != "defense" {
		t.Fatalf("phase = %s, want defense after attack", next.Phase)
	}
}

func TestLocalGameSubmitActionRejectsUnknownID(t *testing.T) {
	game := mustLocalGame(t, domain.Seat(0), firstLegalControllers(domain.Seat(1)))

	state, err := game.SubmitAction(context.Background(), "99")
	if !errors.Is(err, ErrUnknownActionID) {
		t.Fatalf("SubmitAction error = %v, want ErrUnknownActionID", err)
	}
	if state.Version != 0 || state.Phase != "attack" {
		t.Fatalf("state = %+v, want unchanged attack state", state)
	}
}

func TestLocalGameAdvanceRunsControllerUntilHumanCanAct(t *testing.T) {
	controllers := map[domain.Seat]app.PlayerController{
		domain.Seat(0): controllerFunc(func(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
			if turn.SeriesID != "series-1" || turn.MatchID != "match-1" || turn.MatchNumber != 1 || turn.TurnNumber != 1 {
				t.Fatalf("turn context = %+v, want first match turn", turn)
			}
			return app.ActionDecision(turn.LegalActions[0]), nil
		}),
	}
	game := mustLocalGame(t, domain.Seat(1), controllers)
	if state := game.State(); len(state.LegalActions) != 0 {
		t.Fatalf("initial human actions = %+v, want bot to act first", state.LegalActions)
	}

	state, err := game.Advance(context.Background())
	if err != nil {
		t.Fatalf("Advance returned error: %v", err)
	}

	if state.Version != 1 || state.Phase != "defense" || len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want human defense actions after bot attack", state)
	}
}

func TestLocalGameConcedeAndNextMatch(t *testing.T) {
	game := mustLocalGame(t, domain.Seat(0), firstLegalControllers(domain.Seat(1)))

	complete, err := game.Concede(context.Background())
	if err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	if complete.Phase != "complete" || complete.Winner != 1 || complete.Loser != 0 {
		t.Fatalf("complete state = %+v, want winner 1 loser 0", complete)
	}

	next, err := game.NextMatch(context.Background())
	if err != nil {
		t.Fatalf("NextMatch returned error: %v", err)
	}
	if next.MatchID != "match-1-2" || next.Phase == "complete" {
		t.Fatalf("next state = %+v, want active match-1-2", next)
	}
	if next.Version != complete.Version+1 {
		t.Fatalf("version = %d, want %d", next.Version, complete.Version+1)
	}
}

func TestLocalGameNextMatchCanRetryAfterStartFailure(t *testing.T) {
	deal := fixedDeal()
	shuffler := deal.Shuffler
	deals := 0
	var cancel context.CancelFunc
	deal.Shuffler = domain.ShuffleFunc(func(cards []domain.Card) {
		shuffler.Shuffle(cards)
		if deals == 1 && cancel != nil {
			cancel()
		}
		deals++
	})
	game := mustLocalGameWithDeal(t, domain.Seat(0), firstLegalControllers(domain.Seat(1)), deal)
	complete, err := game.Concede(context.Background())
	if err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	failed, err := game.NextMatch(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("NextMatch canceled error = %v, want context.Canceled", err)
	}
	if failed.MatchID != complete.MatchID || failed.Phase != "complete" {
		t.Fatalf("failed state = %+v, want completed original match", failed)
	}

	next, err := game.NextMatch(context.Background())
	if err != nil {
		t.Fatalf("NextMatch retry returned error: %v", err)
	}
	if next.MatchID != "match-1-2" || next.Phase == "complete" {
		t.Fatalf("next state = %+v, want active match-1-2 after retry", next)
	}
}

func TestLocalGameNextMatchRequiresCompletedMatch(t *testing.T) {
	game := mustLocalGame(t, domain.Seat(0), firstLegalControllers(domain.Seat(1)))

	_, err := game.NextMatch(context.Background())
	if !errors.Is(err, ErrMatchInProgress) {
		t.Fatalf("NextMatch error = %v, want ErrMatchInProgress", err)
	}
}

func TestNewLocalGameRejectsMissingController(t *testing.T) {
	_, err := NewLocalGame(context.Background(), &LocalGameOptions{
		SeriesID:    "series-1",
		BaseMatchID: "match-1",
		PlayerCount: 2,
		HumanSeat:   domain.Seat(0),
		Deal:        fixedDeal(),
	})
	if !errors.Is(err, app.ErrMissingPlayerController) {
		t.Fatalf("NewLocalGame error = %v, want ErrMissingPlayerController", err)
	}
}

type controllerFunc func(context.Context, *app.TurnContext) (app.PlayerDecision, error)

func (fn controllerFunc) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	return fn(ctx, turn)
}

func firstLegalControllers(seats ...domain.Seat) map[domain.Seat]app.PlayerController {
	controllers := make(map[domain.Seat]app.PlayerController, len(seats))
	for _, seat := range seats {
		controllers[seat] = controllerFunc(func(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
			return app.ActionDecision(turn.LegalActions[0]), nil
		})
	}
	return controllers
}

func mustLocalGame(
	t *testing.T,
	humanSeat domain.Seat,
	controllers map[domain.Seat]app.PlayerController,
) *LocalGame {
	t.Helper()
	game, err := NewLocalGame(context.Background(), &LocalGameOptions{
		SeriesID:    "series-1",
		BaseMatchID: "match-1",
		PlayerCount: 2,
		HumanSeat:   humanSeat,
		Deal:        fixedDeal(),
		Controllers: controllers,
	})
	if err != nil {
		t.Fatalf("NewLocalGame returned error: %v", err)
	}
	return game
}

func mustLocalGameWithDeal(
	t *testing.T,
	humanSeat domain.Seat,
	controllers map[domain.Seat]app.PlayerController,
	deal domain.DealOptions,
) *LocalGame {
	t.Helper()
	game, err := NewLocalGame(context.Background(), &LocalGameOptions{
		SeriesID:    "series-1",
		BaseMatchID: "match-1",
		PlayerCount: 2,
		HumanSeat:   humanSeat,
		Deal:        deal,
		Controllers: controllers,
	})
	if err != nil {
		t.Fatalf("NewLocalGame returned error: %v", err)
	}
	return game
}

func fixedDeal() domain.DealOptions {
	hands := [][]domain.Card{
		{
			{Rank: domain.Six, Suit: domain.Hearts},
			{Rank: domain.Ten, Suit: domain.Clubs},
			{Rank: domain.Jack, Suit: domain.Diamonds},
			{Rank: domain.Queen, Suit: domain.Clubs},
			{Rank: domain.King, Suit: domain.Spades},
			{Rank: domain.Ace, Suit: domain.Diamonds},
		},
		{
			{Rank: domain.Eight, Suit: domain.Hearts},
			{Rank: domain.Six, Suit: domain.Clubs},
			{Rank: domain.Seven, Suit: domain.Diamonds},
			{Rank: domain.Eight, Suit: domain.Clubs},
			{Rank: domain.Nine, Suit: domain.Spades},
			{Rank: domain.Queen, Suit: domain.Diamonds},
		},
	}
	deck := deckForDeal(hands, stockWithBottom(domain.Card{Rank: domain.Nine, Suit: domain.Hearts}, hands...))
	return domain.DealOptions{Shuffler: copyDeck(deck)}
}

func deckForDeal(hands [][]domain.Card, stock []domain.Card) []domain.Card {
	deck := make([]domain.Card, 0, len(hands)*len(hands[0])+len(stock))
	for cardIndex := range len(hands[0]) {
		for seat := range hands {
			deck = append(deck, hands[seat][cardIndex])
		}
	}
	return append(deck, stock...)
}

func stockWithBottom(bottom domain.Card, hands ...[]domain.Card) []domain.Card {
	stock := make([]domain.Card, 0, 24)
	for _, card := range domain.NewDeck36() {
		if card == bottom || handsContain(hands, card) {
			continue
		}
		stock = append(stock, card)
	}
	return append(stock, bottom)
}

func handsContain(hands [][]domain.Card, card domain.Card) bool {
	for _, hand := range hands {
		if slices.Contains(hand, card) {
			return true
		}
	}
	return false
}

func copyDeck(deck []domain.Card) domain.ShuffleFunc {
	return func(cards []domain.Card) {
		copy(cards, deck)
	}
}
