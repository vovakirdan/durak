package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
)

const defaultMaxRawCommandAttempts = 1

var (
	// ErrNilClient means an AI controller was created without a client.
	ErrNilClient = errors.New("nil ai client")
	// ErrInvalidRawCommand means an AI client returned no usable player command.
	ErrInvalidRawCommand = errors.New("invalid raw ai command")
)

// RawCommandControllerOptions configures raw-command AI player behavior.
type RawCommandControllerOptions struct {
	Client      Client
	MaxAttempts int
	Fallback    app.PlayerController
	TraceSink   RawCommandTraceSink
}

// RawCommandController asks an AI client for terminal-style commands.
type RawCommandController struct {
	client      Client
	maxAttempts int
	fallback    app.PlayerController
	traceSink   RawCommandTraceSink
}

// NewRawCommandController creates a raw-command AI player controller.
func NewRawCommandController(options RawCommandControllerOptions) (*RawCommandController, error) {
	if options.Client == nil {
		return nil, ErrNilClient
	}
	maxAttempts := options.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxRawCommandAttempts
	}
	return &RawCommandController{
		client:      options.Client,
		maxAttempts: maxAttempts,
		fallback:    options.Fallback,
		traceSink:   options.TraceSink,
	}, nil
}

// Decide returns a player decision parsed from an AI raw command response.
func (c *RawCommandController) Decide(ctx context.Context, turn *app.TurnContext) (app.PlayerDecision, error) {
	if c == nil || c.client == nil {
		return app.PlayerDecision{}, ErrNilClient
	}
	if turn == nil {
		return app.PlayerDecision{}, app.ErrNilTurn
	}

	previousErrors := make([]string, 0)
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		prompt := buildRawCommandPrompt(turn, attempt, previousErrors)
		startedAt := time.Now()
		trace := RawCommandTrace{
			Prompt:    cloneTurnPrompt(&prompt),
			Client:    clientInfo(c.client),
			StartedAt: startedAt,
		}
		response, err := c.client.CompleteTurn(ctx, &prompt)
		trace.Duration = time.Since(startedAt)
		trace.RawCommand = response.TextCommand
		trace.Usage = response.Usage
		if err != nil {
			trace.Err = err.Error()
			c.recordTrace(&trace)
			return app.PlayerDecision{}, err
		}

		decision, parseErr := c.parseResponse(response, turn, &trace)
		c.recordTrace(&trace)
		if parseErr == nil {
			return decision, nil
		}
		previousErrors = append(previousErrors, parseErr.Error())
	}

	if c.fallback != nil {
		return c.fallback.Decide(ctx, turn)
	}
	return app.PlayerDecision{}, fmt.Errorf("%w: %s", ErrInvalidRawCommand, previousErrors[len(previousErrors)-1])
}

func (c *RawCommandController) parseResponse(
	response TurnResponse,
	turn *app.TurnContext,
	trace *RawCommandTrace,
) (app.PlayerDecision, error) {
	command, err := textcmd.Parse(response.TextCommand, &turn.DecisionContext)
	if err != nil {
		trace.Err = err.Error()
		return app.PlayerDecision{}, fmt.Errorf("%w: %w", ErrInvalidRawCommand, err)
	}
	trace.CommandKind = command.Kind

	switch command.Kind {
	case textcmd.KindAction:
		decision := app.ActionDecision(command.Action)
		trace.Decision = decision
		return decision, nil
	case textcmd.KindConcede:
		if !turn.CanConcede {
			err := fmt.Errorf("%w: concede is not available", app.ErrInvalidPlayerDecision)
			trace.Err = err.Error()
			return app.PlayerDecision{}, fmt.Errorf("%w: %w", ErrInvalidRawCommand, err)
		}
		decision := app.ConcedeDecision()
		trace.Decision = decision
		return decision, nil
	case textcmd.KindHelp, textcmd.KindQuit:
		err := fmt.Errorf("non-player command kind %d", command.Kind)
		trace.Err = err.Error()
		return app.PlayerDecision{}, fmt.Errorf("%w: %w", ErrInvalidRawCommand, err)
	default:
		err := fmt.Errorf("unknown command kind %d", command.Kind)
		trace.Err = err.Error()
		return app.PlayerDecision{}, fmt.Errorf("%w: %w", ErrInvalidRawCommand, err)
	}
}

func (c *RawCommandController) recordTrace(trace *RawCommandTrace) {
	if c.traceSink != nil {
		c.traceSink.RecordRawCommandTrace(trace)
	}
}
