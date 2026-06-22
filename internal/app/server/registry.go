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
	// ErrSeatOccupied means a human seat already has a live session.
	ErrSeatOccupied = errors.New("seat occupied")
	// ErrStaleState means the caller used an older state version.
	ErrStaleState = errors.New("stale state")
)

// Registry owns in-memory tables for a future daemon boundary.
type Registry struct {
	mu     sync.Mutex
	tables map[string]*table
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

// CreateTable registers one in-memory table under id and returns its first human state.
func (r *Registry) CreateTable(ctx context.Context, id string, options *TableOptions) (client.State, error) {
	if r == nil || id == "" || options == nil {
		return client.State{}, ErrInvalidTable
	}
	created, err := newTable(ctx, options)
	if err != nil {
		return client.State{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tables[id]; ok {
		return client.State{}, fmt.Errorf("%w: %s", ErrTableExists, id)
	}
	r.tables[id] = created
	return created.stateForSeat(created.firstHumanSeat()), nil
}

// JoinTable reserves one human seat and returns its current state.
func (r *Registry) JoinTable(id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.joinSeat(seat)
}

// ReleaseSeat frees a previously joined human seat.
func (r *Registry) ReleaseSeat(id string, seat domain.Seat) error {
	t, err := r.table(id)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.releaseSeat(seat)
}

// State returns the current table state for one seat.
func (r *Registry) State(id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state(seat)
}

// Advance runs controller turns and returns the current table state for one seat.
func (r *Registry) Advance(ctx context.Context, id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.advance(ctx, seat)
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
	return t.submitAction(ctx, seat, version, actionID)
}

// Concede gives up the current match for the joined table seat.
func (r *Registry) Concede(ctx context.Context, id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.concede(ctx, seat)
}

// NextMatch starts the next match for the joined table seat.
func (r *Registry) NextMatch(ctx context.Context, id string, seat domain.Seat) (client.State, error) {
	t, err := r.table(id)
	if err != nil {
		return client.State{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.nextMatch(ctx, seat)
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
