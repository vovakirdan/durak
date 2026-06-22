package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

// TableGame adapts a registry table to the TUI game contract.
type TableGame struct {
	mu       sync.Mutex
	registry *Registry
	tableID  string
	seat     domain.Seat
	closed   bool
}

// NewTableGame joins one registry table seat.
func NewTableGame(registry *Registry, tableID string, seat domain.Seat) (*TableGame, error) {
	if registry == nil || tableID == "" {
		return nil, ErrTableNotFound
	}
	if _, err := registry.JoinTable(tableID, seat); err != nil {
		return nil, err
	}
	return &TableGame{registry: registry, tableID: tableID, seat: seat}, nil
}

// State returns the current table state, or an empty state if the table vanished.
func (g *TableGame) State() client.State {
	if g == nil {
		return client.State{}
	}
	state, err := g.registry.State(g.tableID, g.seat)
	if err != nil {
		return client.State{}
	}
	return state
}

// Advance refreshes the joined table state.
func (g *TableGame) Advance(ctx context.Context) (client.State, error) {
	if g == nil {
		return client.State{}, fmt.Errorf("%w: nil table game", ErrTableNotFound)
	}
	return g.registry.Advance(ctx, g.tableID, g.seat)
}

// SubmitAction applies an action against the latest table version.
func (g *TableGame) SubmitAction(ctx context.Context, actionID string) (client.State, error) {
	if g == nil {
		return client.State{}, fmt.Errorf("%w: nil table game", ErrTableNotFound)
	}
	state, err := g.registry.State(g.tableID, g.seat)
	if err != nil {
		return state, err
	}
	return g.registry.SubmitAction(ctx, g.tableID, g.seat, state.Version, actionID)
}

// Concede gives up the current table match.
func (g *TableGame) Concede(ctx context.Context) (client.State, error) {
	if g == nil {
		return client.State{}, fmt.Errorf("%w: nil table game", ErrTableNotFound)
	}
	return g.registry.Concede(ctx, g.tableID, g.seat)
}

// NextMatch starts the next table match.
func (g *TableGame) NextMatch(ctx context.Context) (client.State, error) {
	if g == nil {
		return client.State{}, fmt.Errorf("%w: nil table game", ErrTableNotFound)
	}
	return g.registry.NextMatch(ctx, g.tableID, g.seat)
}

// Close releases the table seat held by this game.
func (g *TableGame) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return nil
	}
	g.closed = true
	return g.registry.ReleaseSeat(g.tableID, g.seat)
}
