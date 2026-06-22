package server

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRegistryCreatesAndJoinsTable(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	joined, err := registry.JoinTable("table-1", domain.Seat(state.Seat))
	if err != nil {
		t.Fatalf("JoinTable returned error: %v", err)
	}

	if joined.MatchID != state.MatchID || joined.Version != state.Version {
		t.Fatalf("joined = %+v, want initial state %+v", joined, state)
	}
}

func TestRegistryRejectsDuplicateTable(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testGame(t)); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	_, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if !errors.Is(err, ErrTableExists) {
		t.Fatalf("CreateTable duplicate error = %v, want ErrTableExists", err)
	}
}

func TestRegistryRejectsUnavailableSeat(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testGame(t)); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	_, err := registry.JoinTable("table-1", domain.Seat(1))
	if !errors.Is(err, ErrSeatUnavailable) {
		t.Fatalf("JoinTable error = %v, want ErrSeatUnavailable", err)
	}
}

func TestRegistrySubmitsAction(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	if len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want legal action", state)
	}

	next, err := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(state.Seat),
		state.Version,
		state.LegalActions[0].ID,
	)
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}

	if next.Version <= state.Version {
		t.Fatalf("next version = %d, want > %d", next.Version, state.Version)
	}
}

func TestRegistryRejectsStaleActionVersion(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	if _, submitErr := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(state.Seat),
		state.Version,
		state.LegalActions[0].ID,
	); submitErr != nil {
		t.Fatalf("SubmitAction first returned error: %v", submitErr)
	}

	_, err = registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(state.Seat),
		state.Version,
		state.LegalActions[0].ID,
	)
	if !errors.Is(err, ErrStaleState) {
		t.Fatalf("SubmitAction stale error = %v, want ErrStaleState", err)
	}
}

func TestRegistryReturnsCurrentStateOnSubmitError(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	got, err := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(state.Seat),
		state.Version,
		"999",
	)
	if !errors.Is(err, client.ErrUnknownActionID) {
		t.Fatalf("SubmitAction error = %v, want ErrUnknownActionID", err)
	}
	if got.MatchID != state.MatchID || got.Version != state.Version || got.Phase != state.Phase {
		t.Fatalf("state = %+v, want original %+v", got, state)
	}
}

func TestRegistryConcedeAndNextMatch(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	complete, err := registry.Concede(context.Background(), "table-1", domain.Seat(state.Seat))
	if err != nil {
		t.Fatalf("Concede returned error: %v", err)
	}
	if complete.Phase != "complete" {
		t.Fatalf("phase = %q, want complete", complete.Phase)
	}

	next, err := registry.NextMatch(context.Background(), "table-1", domain.Seat(state.Seat))
	if err != nil {
		t.Fatalf("NextMatch returned error: %v", err)
	}
	if next.MatchID == complete.MatchID || next.Phase == "complete" {
		t.Fatalf("next = %+v, want active next match", next)
	}
}

func TestTableGameUsesSharedRegistryTable(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testGame(t))
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	game, err := NewTableGame(registry, "table-1", domain.Seat(state.Seat))
	if err != nil {
		t.Fatalf("NewTableGame returned error: %v", err)
	}
	if len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want legal actions", state)
	}

	next, err := game.SubmitAction(context.Background(), state.LegalActions[0].ID)
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}
	joined, err := registry.JoinTable("table-1", domain.Seat(state.Seat))
	if err != nil {
		t.Fatalf("JoinTable returned error: %v", err)
	}
	if joined.Version != next.Version {
		t.Fatalf("joined version = %d, want %d", joined.Version, next.Version)
	}
}

func testGame(t *testing.T) *client.LocalGame {
	t.Helper()
	game, err := client.NewLocalGame(context.Background(), &client.LocalGameOptions{
		SeriesID:    "server-series",
		BaseMatchID: "server-match",
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
