package tui

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestNewModelInitializesState(t *testing.T) {
	model := NewModel(context.Background(), testGame(t))

	if model.err != nil {
		t.Fatalf("NewModel error = %v", model.err)
	}
	if model.state.MatchID == "" || model.state.Phase == "" {
		t.Fatalf("state = %+v, want initialized game state", model.state)
	}
}

func TestViewRendersPlayableState(t *testing.T) {
	model := &Model{state: client.State{
		MatchID:        "match-1",
		Version:        2,
		Seat:           0,
		Phase:          "defense",
		Attacker:       1,
		Defender:       0,
		TrumpSuit:      "H",
		TrumpIndicator: client.Card{Code: "9H"},
		Table:          []client.TablePair{{Attack: client.Card{Code: "6C"}}},
		Hand:           []client.Card{{Code: "7C"}},
		HandSizes:      []int{1, 1},
		StockCount:     20,
		DiscardCount:   2,
		LegalActions:   []client.LegalAction{{ID: "1", Label: "defend 1 7C"}},
	}}

	view := model.View().Content
	for _, want := range []string{
		"Durak TUI",
		"Match: match-1 v2 | Phase: defense",
		"Trump: H (9H)",
		"1. 6C / --",
		"Hand: 1:7C",
		"1. defend 1 7C",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() = %q, want %q", view, want)
		}
	}
}

func TestViewRendersWaitingState(t *testing.T) {
	model := &Model{state: client.State{
		MatchID:  "match-1",
		Phase:    "defense",
		Seat:     0,
		Attacker: 0,
		Defender: 1,
	}}

	view := model.View().Content
	if !strings.Contains(view, "Actions: waiting for another seat") {
		t.Fatalf("View() = %q, want waiting message", view)
	}
}

func TestUpdateSubmitsNumberedAction(t *testing.T) {
	model := NewModel(context.Background(), testGame(t))
	if len(model.state.LegalActions) == 0 {
		t.Fatalf("initial state = %+v, want legal actions", model.state)
	}
	before := model.state.Version

	updated, _ := model.Update(keyPress("1"))
	next := updated.(*Model)

	if next.err != nil {
		t.Fatalf("Update error = %v", next.err)
	}
	if next.state.Version <= before {
		t.Fatalf("version = %d, want > %d", next.state.Version, before)
	}
}

func TestUpdateHandlesPastedKeys(t *testing.T) {
	model := NewModel(context.Background(), testGame(t))

	_, cmd := model.Update(keyPress("1q"))

	if cmd == nil {
		t.Fatal("cmd = nil, want quit command after pasted action and q")
	}
}

func TestUpdateBuffersAmbiguousActionID(t *testing.T) {
	model := &Model{state: client.State{
		LegalActions: []client.LegalAction{
			{ID: "1", Label: "attack 6C"},
			{ID: "10", Label: "take"},
		},
	}}

	updated, cmd := model.Update(keyPress("1"))
	next := updated.(*Model)

	if cmd != nil {
		t.Fatalf("cmd = %v, want nil while action id is ambiguous", cmd)
	}
	if next.actionInput != "1" {
		t.Fatalf("actionInput = %q, want buffered 1", next.actionInput)
	}
	if next.err != nil {
		t.Fatalf("err = %v, want nil before submit", next.err)
	}
}

func TestUpdateSubmitsBufferedActionOnEnter(t *testing.T) {
	model := &Model{
		state: client.State{
			LegalActions: []client.LegalAction{
				{ID: "1", Label: "attack 6C"},
				{ID: "10", Label: "take"},
			},
		},
		actionInput: "1",
	}

	updated, _ := model.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	next := updated.(*Model)

	if next.actionInput != "" {
		t.Fatalf("actionInput = %q, want cleared after enter", next.actionInput)
	}
	if next.err == nil {
		t.Fatal("err = nil, want submit attempt to reach missing game")
	}
}

func TestUpdateHandlesNilGameCommands(t *testing.T) {
	for _, input := range []string{"c", "n"} {
		model := NewModel(context.Background(), nil)
		model.state.Phase = "complete"

		updated, _ := model.Update(keyPress(input))
		next := updated.(*Model)

		if !errors.Is(next.err, client.ErrInvalidLocalGame) {
			t.Fatalf("key %q err = %v, want ErrInvalidLocalGame", input, next.err)
		}
	}
}

func TestUpdateClosesGameOnQuit(t *testing.T) {
	game := &closeSpyGame{state: client.State{MatchID: "match-1"}}
	model := &Model{ctx: context.Background(), game: game, state: game.state}

	_, cmd := model.Update(keyPress("q"))

	if cmd == nil {
		t.Fatal("cmd = nil, want quit command")
	}
	if !game.closed {
		t.Fatal("closed = false, want close on quit")
	}
}

func TestUpdateRefreshesOnTick(t *testing.T) {
	game := &closeSpyGame{state: client.State{MatchID: "match-1", Version: 1}}
	model := &Model{ctx: context.Background(), game: game, state: game.state}

	updated, cmd := model.Update(refreshMsg(time.Now()))
	next := updated.(*Model)

	if cmd == nil {
		t.Fatal("cmd = nil, want next refresh tick")
	}
	if game.advanceCalls != 1 {
		t.Fatalf("advance calls = %d, want 1", game.advanceCalls)
	}
	if next.state.Version != 2 {
		t.Fatalf("version = %d, want refreshed version 2", next.state.Version)
	}
}

func TestUpdateStartsNextMatchAfterComplete(t *testing.T) {
	game := testGame(t)
	complete, err := game.Concede(context.Background())
	if err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	model := &Model{ctx: context.Background(), game: game, state: complete}

	updated, _ := model.Update(keyPress("n"))
	next := updated.(*Model)

	if next.err != nil {
		t.Fatalf("Update error = %v", next.err)
	}
	if next.state.MatchID != "tui-test-2" || next.state.Phase == "complete" {
		t.Fatalf("state = %+v, want active second match", next.state)
	}
}

func keyPress(input string) tea.KeyPressMsg {
	runes := []rune(input)
	key := tea.KeyPressMsg{Text: input}
	if len(runes) == 1 {
		key.Code = runes[0]
	}
	return key
}

func testGame(t *testing.T) *client.LocalGame {
	t.Helper()
	game, err := client.NewLocalGame(context.Background(), &client.LocalGameOptions{
		SeriesID:    "tui-series",
		BaseMatchID: "tui-test",
		PlayerCount: 2,
		HumanSeat:   domain.Seat(0),
		Deal:        domain.SeededDealOptions(42),
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(1): firstLegalController{},
		},
	})
	if err != nil {
		t.Fatalf("NewLocalGame returned error: %v", err)
	}
	return game
}

type firstLegalController struct{}

func (firstLegalController) Decide(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	return app.ActionDecision(turn.LegalActions[0]), nil
}

type closeSpyGame struct {
	state        client.State
	closed       bool
	advanceCalls int
}

func (g *closeSpyGame) State() client.State {
	return g.state
}

func (g *closeSpyGame) SubmitAction(context.Context, string) (client.State, error) {
	g.state.Version++
	return g.state, nil
}

func (g *closeSpyGame) Advance(context.Context) (client.State, error) {
	g.advanceCalls++
	g.state.Version++
	return g.state, nil
}

func (g *closeSpyGame) Concede(context.Context) (client.State, error) {
	g.state.Phase = "complete"
	return g.state, nil
}

func (g *closeSpyGame) NextMatch(context.Context) (client.State, error) {
	g.state.MatchID = "match-2"
	g.state.Phase = "attack"
	return g.state, nil
}

func (g *closeSpyGame) Close() error {
	g.closed = true
	return nil
}
