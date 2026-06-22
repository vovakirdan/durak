package server

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

var (
	// ErrTableExists means a table id is already registered.
	ErrTableExists = errors.New("table exists")
	// ErrTableNotFound means a table id is absent.
	ErrTableNotFound = errors.New("table not found")
	// ErrInvalidTable means table creation input is unusable.
	ErrInvalidTable = errors.New("invalid table")
	// ErrSeatUnavailable means a seat cannot join or act in this table.
	ErrSeatUnavailable = errors.New("seat unavailable")
	// ErrStaleState means the caller used an older state version.
	ErrStaleState = errors.New("stale state")
)

// Registry owns in-memory tables for a future daemon boundary.
type Registry struct {
	mu     sync.Mutex
	tables map[string]*table
}

type table struct {
	mu   sync.Mutex
	game *client.LocalGame
	seat domain.Seat
}

// NewRegistry creates an empty in-memory table registry.
func NewRegistry() *Registry {
	return &Registry{tables: make(map[string]*table)}
}

// TableCount returns the number of registered tables.
func (r *Registry) TableCount() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tables)
}

// CreateTable registers one LocalGame under id and returns its playable state.
func (r *Registry) CreateTable(ctx context.Context, id string, game *client.LocalGame) (client.State, error) {
	if r == nil || id == "" || game == nil {
		return client.State{}, ErrInvalidTable
	}
	state, err := game.Advance(ctx)
	if err != nil {
		return state, err
	}
	created := &table{
		game: game,
		seat: domain.Seat(state.Seat),
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tables[id]; ok {
		return client.State{}, fmt.Errorf("%w: %s", ErrTableExists, id)
	}
	r.tables[id] = created
	return state, nil
}

// JoinTable returns the current state for the table's playable seat.
func (r *Registry) JoinTable(id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if seat != t.seat {
		return client.State{}, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	return t.game.State(), nil
}

// SubmitAction applies an action only if the caller's state version is current.
func (r *Registry) SubmitAction(
	ctx context.Context,
	id string,
	seat domain.Seat,
	version uint64,
	actionID string,
) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.game.State()
	if seat != t.seat {
		return state, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	if state.Version != version {
		return state, fmt.Errorf("%w: got %d want %d", ErrStaleState, version, state.Version)
	}
	if _, err := t.game.SubmitAction(ctx, actionID); err != nil {
		return state, err
	}
	return t.game.Advance(ctx)
}

// Concede gives up the current match for the joined table seat.
func (r *Registry) Concede(ctx context.Context, id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.game.State()
	if seat != t.seat {
		return state, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	return t.game.Concede(ctx)
}

// NextMatch starts the next match for the joined table seat.
func (r *Registry) NextMatch(ctx context.Context, id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.game.State()
	if seat != t.seat {
		return state, fmt.Errorf("%w: seat %d", ErrSeatUnavailable, seat)
	}
	state, err = t.game.NextMatch(ctx)
	if err != nil {
		return state, err
	}
	return t.game.Advance(ctx)
}

func (r *Registry) table(id string) (*table, error) {
	if r == nil || id == "" {
		return nil, ErrTableNotFound
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tables[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrTableNotFound, id)
	}
	return t, nil
}
