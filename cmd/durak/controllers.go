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

const controllerAIRawMock = "ai-raw-mock"

var errUnknownPlayerController = errors.New("unknown player controller")

type playerControllerConfig struct {
	Kind      string
	Seed      uint64
	Seeded    bool
	Seat      domain.Seat
	Fallback  app.PlayerController
	TraceSink ai.RawCommandTraceSink
}

func controllerNames() string {
	return strings.Join([]string{
		bot.ControllerSimple,
		bot.ControllerRandom,
		controllerAIRawMock,
	}, ", ")
}

func newPlayerController(config playerControllerConfig) (app.PlayerController, error) {
	kind := normalizePlayerControllerKind(config.Kind)
	switch kind {
	case bot.ControllerSimple, bot.ControllerRandom:
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
	default:
		return nil, fmt.Errorf("%w: %q", errUnknownPlayerController, kind)
	}
}

func simpleFallbackController() app.PlayerController {
	return app.StrategyController{Strategy: bot.NewSimpleStrategy()}
}

func validatePlayerControllerKind(kind string) error {
	switch normalizePlayerControllerKind(kind) {
	case bot.ControllerSimple, bot.ControllerRandom, controllerAIRawMock:
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
