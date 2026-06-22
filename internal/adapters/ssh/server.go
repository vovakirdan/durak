package ssh

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	tea "charm.land/bubbletea/v2"
	wish "charm.land/wish/v2"
	wishtea "charm.land/wish/v2/bubbletea"
	gossh "github.com/charmbracelet/ssh"
	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/adapters/tui"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/app/server"
	"github.com/vovakirdan/durak/internal/domain"
)

// DefaultAddr is the development SSH listen address.
const DefaultAddr = "localhost:23234"

var (
	// ErrNilOptions means NewServer received nil options.
	ErrNilOptions = errors.New("nil ssh server options")
	// ErrMissingHostKeyPath means NewServer would otherwise create a key in cwd.
	ErrMissingHostKeyPath = errors.New("missing ssh host key path")
	// ErrInvalidSessionCommand means an SSH exec command cannot be mapped to a game session.
	ErrInvalidSessionCommand = errors.New("invalid ssh session command")
)

// GameFactory creates one TUI-driving game for a single SSH session.
type GameFactory func(context.Context, []string) (tui.Game, error)

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
	TableID     string
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
		var err error
		newGame, err = newGameFactory(options)
		if err != nil {
			return nil, err
		}
	}

	sshOptions := []gossh.Option{
		wish.WithAddress(addr),
		wish.WithMiddleware(wishtea.Middleware(func(sess gossh.Session) (tea.Model, []tea.ProgramOption) {
			game, err := newGame(sess.Context(), sess.Command())
			if err != nil {
				return tui.NewErrorModel(err), nil
			}
			return tui.NewModel(sess.Context(), game), nil
		})),
		wish.WithHostKeyPath(options.HostKeyPath),
	}
	return wish.NewServer(sshOptions...)
}

func newGameFactory(options *ServerOptions) (GameFactory, error) {
	gameOptions := options.Game
	if err := validateGameOptions(&gameOptions); err != nil {
		return nil, err
	}
	if options.TableID == "" {
		return func(ctx context.Context, args []string) (tui.Game, error) {
			if len(args) != 0 {
				return nil, fmt.Errorf("%w: %v", ErrInvalidSessionCommand, args)
			}
			return NewLocalGame(ctx, &gameOptions)
		}, nil
	}
	registry := server.NewRegistry()
	var mu sync.Mutex
	created := false
	return func(ctx context.Context, args []string) (tui.Game, error) {
		seat, err := sessionSeat(args)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		defer mu.Unlock()
		if !created {
			tableOptions := sshTableOptions(&gameOptions)
			if _, createErr := registry.CreateTable(ctx, options.TableID, tableOptions); createErr != nil {
				return nil, createErr
			}
			created = true
		}
		game, err := server.NewTableGame(registry, options.TableID, seat)
		if err != nil {
			return nil, err
		}
		releaseOnSessionDone(ctx, game)
		return game, nil
	}, nil
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

func sshTableOptions(options *GameOptions) *server.TableOptions {
	tableOptions := &server.TableOptions{
		SeriesID:    "ssh-series",
		BaseMatchID: "ssh-match",
		PlayerCount: 2,
		HumanSeats:  []domain.Seat{0, 1},
	}
	if options != nil && options.Seeded {
		tableOptions.Deal = domain.SeededDealOptions(options.Seed)
	}
	return tableOptions
}

func sessionSeat(args []string) (domain.Seat, error) {
	if len(args) == 0 {
		return domain.Seat(0), nil
	}
	if len(args) != 2 || args[0] != "seat" {
		return domain.NoSeat, fmt.Errorf("%w: %v", ErrInvalidSessionCommand, args)
	}
	seat, err := strconv.Atoi(args[1])
	if err != nil || seat < 0 {
		return domain.NoSeat, fmt.Errorf("%w: seat %q", ErrInvalidSessionCommand, args[1])
	}
	return domain.Seat(seat), nil
}

func releaseOnSessionDone(ctx context.Context, game interface{ Close() error }) {
	done := ctx.Done()
	if done == nil {
		return
	}
	go func() {
		<-done
		if err := game.Close(); err != nil {
			return
		}
	}()
}
