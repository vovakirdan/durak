package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

	view := model.View()
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

func TestUpdateSubmitsNumberedAction(t *testing.T) {
	model := NewModel(context.Background(), testGame(t))
	if len(model.state.LegalActions) == 0 {
		t.Fatalf("initial state = %+v, want legal actions", model.state)
	}
	before := model.state.Version

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1")})
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

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("1q")})

	if cmd == nil {
		t.Fatal("cmd = nil, want quit command after pasted action and q")
	}
}

func TestUpdateStartsNextMatchAfterComplete(t *testing.T) {
	game := testGame(t)
	complete, err := game.Concede(context.Background())
	if err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	model := &Model{ctx: context.Background(), game: game, state: complete}

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	next := updated.(*Model)

	if next.err != nil {
		t.Fatalf("Update error = %v", next.err)
	}
	if next.state.MatchID != "tui-test-2" || next.state.Phase == "complete" {
		t.Fatalf("state = %+v, want active second match", next.state)
	}
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
