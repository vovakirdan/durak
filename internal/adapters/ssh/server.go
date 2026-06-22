package ssh

import (
	"context"
	"errors"
	"fmt"

	tea "charm.land/bubbletea/v2"
	wish "charm.land/wish/v2"
	wishtea "charm.land/wish/v2/bubbletea"
	gossh "github.com/charmbracelet/ssh"
	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/adapters/tui"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

// DefaultAddr is the development SSH listen address.
const DefaultAddr = "localhost:23234"

var (
	// ErrNilOptions means NewServer received nil options.
	ErrNilOptions = errors.New("nil ssh server options")
	// ErrMissingHostKeyPath means NewServer would otherwise create a key in cwd.
	ErrMissingHostKeyPath = errors.New("missing ssh host key path")
)

// GameFactory creates one local game for a single SSH session.
type GameFactory func(context.Context) (*client.LocalGame, error)

// GameOptions configures the local game created for each SSH session.
type GameOptions struct {
	Bot    string
	Seed   uint64
	Seeded bool
}

// ServerOptions configures the development Wish server.
type ServerOptions struct {
	Addr        string
	HostKeyPath string
	NewGame     GameFactory
	Game        GameOptions
}

// NewServer builds a Wish SSH server that hosts one TUI game per session.
func NewServer(options *ServerOptions) (*gossh.Server, error) {
	if options == nil {
		return nil, ErrNilOptions
	}
	if options.HostKeyPath == "" {
		return nil, ErrMissingHostKeyPath
	}
	addr := options.Addr
	if addr == "" {
		addr = DefaultAddr
	}
	newGame := options.NewGame
	if newGame == nil {
		gameOptions := options.Game
		if err := validateGameOptions(&gameOptions); err != nil {
			return nil, err
		}
		newGame = func(ctx context.Context) (*client.LocalGame, error) {
			return NewLocalGame(ctx, &gameOptions)
		}
	}

	sshOptions := []gossh.Option{
		wish.WithAddress(addr),
		wish.WithMiddleware(wishtea.Middleware(func(sess gossh.Session) (tea.Model, []tea.ProgramOption) {
			game, err := newGame(sess.Context())
			if err != nil {
				return tui.NewErrorModel(err), nil
			}
			return tui.NewModel(sess.Context(), game), nil
		})),
		wish.WithHostKeyPath(options.HostKeyPath),
	}
	return wish.NewServer(sshOptions...)
}

// NewLocalGame creates the default one-human SSH game against a simple bot.
func NewLocalGame(ctx context.Context, options *GameOptions) (*client.LocalGame, error) {
	gameOptions := GameOptions{}
	if options != nil {
		gameOptions = *options
	}
	if err := validateGameOptions(&gameOptions); err != nil {
		return nil, err
	}
	controller, err := bot.NewController(bot.ControllerSpec{
		Kind:   gameOptions.Bot,
		Seed:   gameOptions.Seed,
		Seeded: gameOptions.Seeded,
	}, domain.Seat(1))
	if err != nil {
		return nil, err
	}
	localOptions := client.LocalGameOptions{
		SeriesID:    "ssh-series",
		BaseMatchID: "ssh-match",
		PlayerCount: 2,
		HumanSeat:   domain.Seat(0),
		Controllers: map[domain.Seat]app.PlayerController{
			domain.Seat(1): controller,
		},
	}
	if gameOptions.Seeded {
		localOptions.Deal = domain.SeededDealOptions(gameOptions.Seed)
	}
	game, err := client.NewLocalGame(ctx, &localOptions)
	if err != nil {
		return nil, fmt.Errorf("new local ssh game: %w", err)
	}
	return game, nil
}

func validateGameOptions(options *GameOptions) error {
	if options == nil {
		return nil
	}
	return bot.ValidateControllerKind(options.Bot)
}
