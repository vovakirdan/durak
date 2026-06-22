package ssh

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app/server"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestNewServerBuildsWishServer(t *testing.T) {
	hostKey := filepath.Join(t.TempDir(), "host_ed25519")

	server, err := NewServer(&ServerOptions{
		Addr:        "127.0.0.1:0",
		HostKeyPath: hostKey,
	})
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}

	if server.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q, want 127.0.0.1:0", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("Handler = nil, want Bubble Tea middleware handler")
	}
	if len(server.HostSigners) == 0 {
		t.Fatal("HostSigners = 0, want generated host key")
	}
}

func TestNewServerRejectsNilOptions(t *testing.T) {
	_, err := NewServer(nil)
	if !errors.Is(err, ErrNilOptions) {
		t.Fatalf("error = %v, want ErrNilOptions", err)
	}
}

func TestNewServerRejectsMissingHostKeyPath(t *testing.T) {
	_, err := NewServer(&ServerOptions{})
	if !errors.Is(err, ErrMissingHostKeyPath) {
		t.Fatalf("error = %v, want ErrMissingHostKeyPath", err)
	}
}

func TestNewGameFactoryReusesSharedTable(t *testing.T) {
	factory, err := newGameFactory(&ServerOptions{
		TableID: "table-1",
		Game: GameOptions{
			Bot:    bot.ControllerSimple,
			Seed:   42,
			Seeded: true,
		},
	})
	if err != nil {
		t.Fatalf("newGameFactory returned error: %v", err)
	}
	first, err := factory(context.Background(), nil)
	if err != nil {
		t.Fatalf("first factory call returned error: %v", err)
	}
	second, err := factory(context.Background(), []string{"seat", "1"})
	if err != nil {
		t.Fatalf("second factory call returned error: %v", err)
	}
	state := first.State()
	acting := first
	other := second
	if state.Attacker == 1 {
		acting = second
		other = first
		state = second.State()
	}
	if len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want legal actions", state)
	}
	next, err := acting.SubmitAction(context.Background(), state.LegalActions[0].ID)
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}

	joined := other.State()
	if joined.Version != next.Version {
		t.Fatalf("joined version = %d, want shared version %d", joined.Version, next.Version)
	}
}

func TestNewGameFactoryUsesBotsForNonHumanTableSeats(t *testing.T) {
	factory, err := newGameFactory(&ServerOptions{
		TableID: "table-1",
		Game: GameOptions{
			Bot:    bot.ControllerSimple,
			Seed:   42,
			Seeded: true,
		},
		Table: server.TableOptions{
			PlayerCount: 2,
			HumanSeats:  []domain.Seat{0},
		},
	})
	if err != nil {
		t.Fatalf("newGameFactory returned error: %v", err)
	}
	game, err := factory(context.Background(), nil)
	if err != nil {
		t.Fatalf("factory call returned error: %v", err)
	}
	state := game.State()
	if state.Seat != 0 || len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want playable human seat", state)
	}

	next, err := game.SubmitAction(context.Background(), state.LegalActions[0].ID)
	if err != nil {
		t.Fatalf("SubmitAction returned error: %v", err)
	}
	if next.Seat != 0 || next.Version <= state.Version+1 {
		t.Fatalf("next = %+v, want bot-advanced human state", next)
	}
}

func TestNewGameFactoryRejectsOccupiedSharedTableSeat(t *testing.T) {
	factory, err := newGameFactory(&ServerOptions{
		TableID: "table-1",
		Game: GameOptions{
			Bot:    bot.ControllerSimple,
			Seed:   42,
			Seeded: true,
		},
	})
	if err != nil {
		t.Fatalf("newGameFactory returned error: %v", err)
	}
	if _, callErr := factory(context.Background(), nil); callErr != nil {
		t.Fatalf("first factory call returned error: %v", callErr)
	}

	_, err = factory(context.Background(), []string{"seat", "0"})
	if !errors.Is(err, server.ErrSeatOccupied) {
		t.Fatalf("second factory call error = %v, want ErrSeatOccupied", err)
	}
}

func TestNewGameFactoryReleasesSeatWhenSessionEnds(t *testing.T) {
	factory, err := newGameFactory(&ServerOptions{
		TableID: "table-1",
		Game: GameOptions{
			Bot:    bot.ControllerSimple,
			Seed:   42,
			Seeded: true,
		},
	})
	if err != nil {
		t.Fatalf("newGameFactory returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if _, err := factory(ctx, nil); err != nil {
		t.Fatalf("factory call returned error: %v", err)
	}
	if _, err := factory(context.Background(), nil); !errors.Is(err, server.ErrSeatOccupied) {
		t.Fatalf("second factory call error = %v, want ErrSeatOccupied", err)
	}

	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		game, err := factory(context.Background(), nil)
		if err == nil {
			if closer, ok := game.(interface{ Close() error }); ok {
				if closeErr := closer.Close(); closeErr != nil {
					t.Fatalf("Close returned error: %v", closeErr)
				}
			}
			return
		}
		if !errors.Is(err, server.ErrSeatOccupied) {
			t.Fatalf("factory after cancel error = %v, want retryable ErrSeatOccupied", err)
		}
		if time.Now().After(deadline) {
			t.Fatal("seat remained occupied after session context cancellation")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNewLocalGameCreatesPlayableGame(t *testing.T) {
	game, err := NewLocalGame(context.Background(), &GameOptions{
		Bot:    bot.ControllerRandom,
		Seed:   42,
		Seeded: true,
	})
	if err != nil {
		t.Fatalf("NewLocalGame returned error: %v", err)
	}

	state, err := game.Advance(context.Background())
	if err != nil {
		t.Fatalf("Advance returned error: %v", err)
	}
	if state.MatchID == "" || len(state.LegalActions) == 0 {
		t.Fatalf("state = %+v, want playable state", state)
	}
}

func TestSessionSeat(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{name: "default", want: 0},
		{name: "explicit", args: []string{"seat", "1"}, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sessionSeat(tc.args)
			if err != nil {
				t.Fatalf("sessionSeat returned error: %v", err)
			}
			if int(got) != tc.want {
				t.Fatalf("seat = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestSessionSeatRejectsInvalidCommand(t *testing.T) {
	for _, args := range [][]string{
		{"seat"},
		{"seat", "-1"},
		{"table", "demo"},
	} {
		if _, err := sessionSeat(args); !errors.Is(err, ErrInvalidSessionCommand) {
			t.Fatalf("sessionSeat(%v) error = %v, want ErrInvalidSessionCommand", args, err)
		}
	}
}

func TestNewLocalGameRejectsUnknownBot(t *testing.T) {
	_, err := NewLocalGame(context.Background(), &GameOptions{Bot: "missing"})
	if !errors.Is(err, bot.ErrUnknownController) {
		t.Fatalf("error = %v, want ErrUnknownController", err)
	}
}
