package server

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRegistryCreatesAndJoinsSeatAwareTable(t *testing.T) {
	registry := NewRegistry()
	state, err := registry.CreateTable(context.Background(), "table-1", testTableOptions())
	if err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	if state.Seat != 0 {
		t.Fatalf("created state seat = %d, want first human seat 0", state.Seat)
	}

	seat0, err := registry.JoinTable("table-1", domain.Seat(0))
	if err != nil {
		t.Fatalf("JoinTable seat 0 returned error: %v", err)
	}
	seat1, err := registry.JoinTable("table-1", domain.Seat(1))
	if err != nil {
		t.Fatalf("JoinTable seat 1 returned error: %v", err)
	}

	if seat0.Seat != 0 || seat1.Seat != 1 {
		t.Fatalf("joined seats = %d/%d, want 0/1", seat0.Seat, seat1.Seat)
	}
	if seat0.MatchID != seat1.MatchID || seat0.Version != seat1.Version {
		t.Fatalf("joined states = %+v / %+v, want same match version", seat0, seat1)
	}
}

func TestRegistryRejectsDuplicateTable(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	_, err := registry.CreateTable(context.Background(), "table-1", testTableOptions())
	if !errors.Is(err, ErrTableExists) {
		t.Fatalf("CreateTable duplicate error = %v, want ErrTableExists", err)
	}
}

func TestRegistryRejectsUnavailableSeat(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}

	_, err := registry.JoinTable("table-1", domain.Seat(2))
	if !errors.Is(err, ErrSeatUnavailable) {
		t.Fatalf("JoinTable error = %v, want ErrSeatUnavailable", err)
	}
}

func TestRegistryRejectsOccupiedSeatUntilRelease(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	if _, err := registry.JoinTable("table-1", domain.Seat(0)); err != nil {
		t.Fatalf("JoinTable first returned error: %v", err)
	}

	_, err := registry.JoinTable("table-1", domain.Seat(0))
	if !errors.Is(err, ErrSeatOccupied) {
		t.Fatalf("JoinTable occupied error = %v, want ErrSeatOccupied", err)
	}

	if err := registry.ReleaseSeat("table-1", domain.Seat(0)); err != nil {
		t.Fatalf("ReleaseSeat returned error: %v", err)
	}
	if _, err := registry.JoinTable("table-1", domain.Seat(0)); err != nil {
		t.Fatalf("JoinTable after release returned error: %v", err)
	}
}

func TestRegistrySubmitsActionForJoinedSeat(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	joinAllSeats(t, registry, "table-1")
	state := activeState(t, registry, "table-1")

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
	defender := domain.Seat(next.Defender)
	defenderState, err := registry.State("table-1", defender)
	if err != nil {
		t.Fatalf("State defender returned error: %v", err)
	}
	if defenderState.Seat != int(defender) || len(defenderState.LegalActions) == 0 {
		t.Fatalf("defender state = %+v, want legal actions", defenderState)
	}
}

func TestRegistrySubmitsActionForSecondHumanSeat(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	joinAllSeats(t, registry, "table-1")
	attacker := activeState(t, registry, "table-1")
	if _, err := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(attacker.Seat),
		attacker.Version,
		attacker.LegalActions[0].ID,
	); err != nil {
		t.Fatalf("SubmitAction attacker returned error: %v", err)
	}
	defender := domain.Seat(attacker.Defender)
	seat, err := registry.State("table-1", defender)
	if err != nil {
		t.Fatalf("State defender returned error: %v", err)
	}
	if len(seat.LegalActions) == 0 {
		t.Fatalf("defender state = %+v, want legal actions", seat)
	}

	next, err := registry.SubmitAction(
		context.Background(),
		"table-1",
		defender,
		seat.Version,
		seat.LegalActions[0].ID,
	)
	if err != nil {
		t.Fatalf("SubmitAction defender returned error: %v", err)
	}
	if next.Seat != int(defender) || next.Version <= seat.Version {
		t.Fatalf("next = %+v, want defender with newer version", next)
	}
}

func TestRegistryAdvancesControllerSeats(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testBotTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	state, err := registry.JoinTable("table-1", domain.Seat(0))
	if err != nil {
		t.Fatalf("JoinTable returned error: %v", err)
	}
	if len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want human legal actions", state)
	}

	next, err := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(0),
		state.Version,
		state.LegalActions[0].ID,
	)
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}
	if next.Version <= state.Version+1 {
		t.Fatalf("next version = %d, want controller advance after human action", next.Version)
	}
}

func TestRegistryRejectsStaleActionVersion(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	joinAllSeats(t, registry, "table-1")
	state := activeState(t, registry, "table-1")
	if _, submitErr := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(state.Seat),
		state.Version,
		state.LegalActions[0].ID,
	); submitErr != nil {
		t.Fatalf("SubmitAction first returned error: %v", submitErr)
	}

	_, err := registry.SubmitAction(
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
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	state, err := registry.JoinTable("table-1", domain.Seat(0))
	if err != nil {
		t.Fatalf("JoinTable returned error: %v", err)
	}

	got, err := registry.SubmitAction(
		context.Background(),
		"table-1",
		domain.Seat(0),
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
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	state, err := registry.JoinTable("table-1", domain.Seat(0))
	if err != nil {
		t.Fatalf("JoinTable returned error: %v", err)
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
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	game0, err := NewTableGame(registry, "table-1", domain.Seat(0))
	if err != nil {
		t.Fatalf("NewTableGame seat 0 returned error: %v", err)
	}
	game1, err := NewTableGame(registry, "table-1", domain.Seat(1))
	if err != nil {
		t.Fatalf("NewTableGame seat 1 returned error: %v", err)
	}
	actingGame := game0
	state := game0.State()
	if state.Attacker == 1 {
		actingGame = game1
		state = game1.State()
	}
	if len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want legal actions", state)
	}

	next, err := actingGame.SubmitAction(context.Background(), state.LegalActions[0].ID)
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}
	joined := game1.State()
	if joined.Version != next.Version || joined.Seat != 1 {
		t.Fatalf("joined = %+v, want shared version %d for seat 1", joined, next.Version)
	}
}

func TestTableGameCloseReleasesSeat(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.CreateTable(context.Background(), "table-1", testTableOptions()); err != nil {
		t.Fatalf("CreateTable returned error: %v", err)
	}
	game, err := NewTableGame(registry, "table-1", domain.Seat(0))
	if err != nil {
		t.Fatalf("NewTableGame returned error: %v", err)
	}
	if _, err := NewTableGame(registry, "table-1", domain.Seat(0)); !errors.Is(err, ErrSeatOccupied) {
		t.Fatalf("second NewTableGame error = %v, want ErrSeatOccupied", err)
	}

	if err := game.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := NewTableGame(registry, "table-1", domain.Seat(0)); err != nil {
		t.Fatalf("NewTableGame after close returned error: %v", err)
	}
}

func testTableOptions() *TableOptions {
	return &TableOptions{
		SeriesID:    "server-series",
		BaseMatchID: "server-match",
		PlayerCount: 2,
		HumanSeats:  []domain.Seat{0, 1},
		Deal:        domain.SeededDealOptions(42),
	}
}

func testBotTableOptions() *TableOptions {
	return &TableOptions{
		SeriesID:    "server-series",
		BaseMatchID: "server-match",
		PlayerCount: 2,
		HumanSeats:  []domain.Seat{0},
		Deal:        domain.SeededDealOptions(42),
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(1): firstLegalController{},
		},
	}
}

func joinAllSeats(t *testing.T, registry *Registry, tableID string) {
	t.Helper()
	for _, seat := range []domain.Seat{0, 1} {
		if _, err := registry.JoinTable(tableID, seat); err != nil {
			t.Fatalf("JoinTable seat %d returned error: %v", seat, err)
		}
	}
}

func activeState(t *testing.T, registry *Registry, tableID string) client.State {
	t.Helper()
	state, err := registry.State(tableID, domain.Seat(0))
	if err != nil {
		t.Fatalf("State seat 0 returned error: %v", err)
	}
	active, err := registry.State(tableID, domain.Seat(state.Attacker))
	if err != nil {
		t.Fatalf("State active seat returned error: %v", err)
	}
	if len(active.LegalActions) == 0 {
		t.Fatalf("active state = %+v, want legal actions", active)
	}
	return active
}

type firstLegalController struct{}

func (firstLegalController) Decide(_ context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	return app.ActionDecision(turn.LegalActions[0]), nil
}
