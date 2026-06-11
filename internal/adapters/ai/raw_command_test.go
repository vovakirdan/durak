package ai_test

import (
	"context"
	"errors"
	"testing"

	"github.com/vovakirdan/durak/internal/adapters/ai"
	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestRawCommandControllerParsesActionCommand(t *testing.T) {
	action := domain.Action{
		Kind: domain.ActionKindAttack,
		Seat: domain.Seat(0),
		Card: domain.Card{Rank: domain.Six, Suit: domain.Clubs},
	}
	controller := mustRawController(t, ai.RawCommandControllerOptions{
		Client: ai.ClientFunc(func(context.Context, *ai.TurnPrompt) (ai.TurnResponse, error) {
			return ai.TurnResponse{TextCommand: "attack 6C"}, nil
		}),
	})

	decision, err := controller.Decide(t.Context(), turnWithActions(action))
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Kind != app.PlayerDecisionAction || decision.Action != action {
		t.Fatalf("decision = %+v, want action", decision)
	}
}

func TestRawCommandControllerRetriesInvalidCommand(t *testing.T) {
	action := domain.Action{
		Kind: domain.ActionKindTake,
		Seat: domain.Seat(0),
	}
	trace := ai.NewMemoryTraceSink()
	controller := mustRawController(t, ai.RawCommandControllerOptions{
		MaxAttempts: 2,
		TraceSink:   trace,
		Client: ai.ClientFunc(func(_ context.Context, prompt *ai.TurnPrompt) (ai.TurnResponse, error) {
			if prompt.Attempt == 1 {
				return ai.TurnResponse{TextCommand: "definitely illegal"}, nil
			}
			if len(prompt.PreviousErrors) != 1 {
				t.Fatalf("PreviousErrors = %v, want one parse error", prompt.PreviousErrors)
			}
			return ai.TurnResponse{TextCommand: "take"}, nil
		}),
	})

	decision, err := controller.Decide(t.Context(), turnWithActions(action))
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Action != action {
		t.Fatalf("decision = %+v, want take action", decision)
	}
	traces := trace.Traces()
	if len(traces) != 2 {
		t.Fatalf("traces = %+v, want two attempts", traces)
	}
	if traces[0].Err == "" || traces[1].CommandKind != textcmd.KindAction {
		t.Fatalf("traces = %+v, want failed then action trace", traces)
	}
}

func TestRawCommandControllerRejectsNonPlayerCommand(t *testing.T) {
	controller := mustRawController(t, ai.RawCommandControllerOptions{
		Client: ai.ClientFunc(func(context.Context, *ai.TurnPrompt) (ai.TurnResponse, error) {
			return ai.TurnResponse{TextCommand: "quit"}, nil
		}),
	})

	_, err := controller.Decide(t.Context(), turnWithActions(domain.Action{Kind: domain.ActionKindTake, Seat: 0}))
	if !errors.Is(err, ai.ErrInvalidRawCommand) {
		t.Fatalf("Decide error = %v, want ErrInvalidRawCommand", err)
	}
}

func TestRawCommandPromptCopiesTurnData(t *testing.T) {
	card := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	action := domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(0), Card: card}
	turn := turnWithActions(action)
	trace := ai.NewMemoryTraceSink()
	controller := mustRawController(t, ai.RawCommandControllerOptions{
		TraceSink: trace,
		Client: ai.ClientFunc(func(_ context.Context, prompt *ai.TurnPrompt) (ai.TurnResponse, error) {
			prompt.Hand[0] = domain.Card{Rank: domain.Ace, Suit: domain.Spades}
			prompt.LegalActions[0].Action.Card = domain.Card{Rank: domain.King, Suit: domain.Hearts}
			prompt.View.HandSizes[0] = 99
			return ai.TurnResponse{TextCommand: "attack 6C"}, nil
		}),
	})

	if _, err := controller.Decide(t.Context(), turn); err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if turn.Hand[0] != card {
		t.Fatalf("turn hand = %v, want original card", turn.Hand)
	}
	if turn.LegalActions[0] != action {
		t.Fatalf("turn legal actions = %v, want original action", turn.LegalActions)
	}
	if turn.HandSizes[0] != 1 {
		t.Fatalf("turn hand sizes = %v, want original size", turn.HandSizes)
	}
	traces := trace.Traces()
	if len(traces) != 1 {
		t.Fatalf("traces = %+v, want one trace", traces)
	}
	if traces[0].Prompt.Hand[0] != card || traces[0].Prompt.LegalActions[0].Action != action {
		t.Fatalf("trace prompt = %+v, want original prompt data", traces[0].Prompt)
	}
}

func mustRawController(t *testing.T, options ai.RawCommandControllerOptions) *ai.RawCommandController {
	t.Helper()
	controller, err := ai.NewRawCommandController(options)
	if err != nil {
		t.Fatalf("NewRawCommandController returned error: %v", err)
	}
	return controller
}

func turnWithActions(actions ...domain.Action) *app.TurnContext {
	hand := make([]domain.Card, 0, len(actions))
	for _, action := range actions {
		if action.Card.Rank != domain.RankUnknown {
			hand = append(hand, action.Card)
		}
	}
	return &app.TurnContext{
		CanConcede: true,
		DecisionContext: app.DecisionContext{
			SeatView: app.SeatView{
				Seat:      domain.Seat(0),
				TrumpSuit: domain.Hearts,
				HandSizes: []int{1, 1},
			},
			Hand:         hand,
			LegalActions: actions,
		},
	}
}
