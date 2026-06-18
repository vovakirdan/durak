package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vovakirdan/durak/internal/adapters/ai"
	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	controllerAIRawMock = "ai-raw-mock"
	controllerAIRawExec = "ai-raw-exec"
	controllerAIOpenAI  = "ai-openai"
)

var (
	errUnknownPlayerController = errors.New("unknown player controller")
	errMissingAIClient         = errors.New("missing ai client")
	errNilPlayerController     = errors.New("nil player controller config")
)

type playerControllerConfig struct {
	Kind      string
	Seed      uint64
	Seeded    bool
	Seat      domain.Seat
	Fallback  app.PlayerController
	TraceSink ai.RawCommandTraceSink
	AI        ai.Client
}

type playControllerOptions struct {
	seats     int
	humanSeat domain.Seat
	botName   string
	players   []string
	seed      uint64
	seeded    bool
	aiConfig  *aiFlags
	traceSink ai.RawCommandTraceSink
}

func controllerNames() string {
	return strings.Join([]string{
		bot.ControllerSimple,
		bot.ControllerRandom,
		bot.ControllerHeuristic,
		controllerAIRawMock,
		controllerAIRawExec,
		controllerAIOpenAI,
	}, ", ")
}

func newPlayerController(config *playerControllerConfig) (app.PlayerController, error) {
	if config == nil {
		return nil, errNilPlayerController
	}
	kind := normalizePlayerControllerKind(config.Kind)
	switch kind {
	case bot.ControllerSimple, bot.ControllerRandom, bot.ControllerHeuristic:
		return bot.NewController(bot.ControllerSpec{
			Kind:   kind,
			Seed:   config.Seed,
			Seeded: config.Seeded,
		}, config.Seat)
	case controllerAIRawMock:
		return ai.NewRawCommandController(ai.RawCommandControllerOptions{
			Client:      ai.NoisyRawCommandClient{},
			MaxAttempts: 2,
			Fallback:    config.Fallback,
			TraceSink:   config.TraceSink,
		})
	case controllerAIRawExec:
		if config.AI == nil {
			return nil, fmt.Errorf("%w: %s requires -ai-command", errMissingAIClient, kind)
		}
		return ai.NewRawCommandController(ai.RawCommandControllerOptions{
			Client:      config.AI,
			MaxAttempts: 2,
			Fallback:    config.Fallback,
			TraceSink:   config.TraceSink,
		})
	case controllerAIOpenAI:
		if config.AI == nil {
			return nil, fmt.Errorf("%w: %s requires -ai-model", errMissingAIClient, kind)
		}
		return ai.NewRawCommandController(ai.RawCommandControllerOptions{
			Client:      config.AI,
			MaxAttempts: 2,
			Fallback:    config.Fallback,
			TraceSink:   config.TraceSink,
		})
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownPlayerController, kind)
	}
}

func playControllers(options *playControllerOptions) (map[domain.Seat]app.PlayerController, error) {
	controllers := make(map[domain.Seat]app.PlayerController, options.seats-1)
	defaultKind := normalizePlayerControllerKind(options.botName)
	for seat := range options.seats {
		domainSeat := domain.Seat(seat)
		if domainSeat == options.humanSeat {
			continue
		}
		kind := defaultKind
		if seat < len(options.players) && options.players[seat] != "" {
			kind = options.players[seat]
		}
		if err := validatePlayerControllerKind(kind); err != nil {
			return nil, fmt.Errorf("p%d: %w", seat, err)
		}
		aiClient, err := options.aiConfig.clientForKind(kind)
		if err != nil {
			return nil, err
		}
		controller, err := newPlayerController(&playerControllerConfig{
			Kind:      kind,
			Seed:      options.seed,
			Seeded:    options.seeded,
			Seat:      domainSeat,
			Fallback:  simpleFallbackController(),
			TraceSink: options.traceSink,
			AI:        aiClient,
		})
		if err != nil {
			return nil, err
		}
		controllers[domainSeat] = controller
	}
	return controllers, nil
}

func simpleFallbackController() app.PlayerController {
	return app.StrategyController{Strategy: bot.NewSimpleStrategy()}
}

func validatePlayerControllerKind(kind string) error {
	switch normalizePlayerControllerKind(kind) {
	case bot.ControllerSimple, bot.ControllerRandom, bot.ControllerHeuristic,
		controllerAIRawMock, controllerAIRawExec, controllerAIOpenAI:
		return nil
	default:
		return fmt.Errorf("%w: %q", errUnknownPlayerController, kind)
	}
}

func normalizePlayerControllerKind(kind string) string {
	if kind == "" {
		return bot.ControllerSimple
	}
	return kind
}
