package ssh

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/bot"
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

func TestNewLocalGameRejectsUnknownBot(t *testing.T) {
	_, err := NewLocalGame(context.Background(), &GameOptions{Bot: "missing"})
	if !errors.Is(err, bot.ErrUnknownController) {
		t.Fatalf("error = %v, want ErrUnknownController", err)
	}
}
