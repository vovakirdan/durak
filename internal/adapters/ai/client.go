package ai

import (
	"context"
	"slices"

	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

// PromptMode identifies how an AI client should answer a turn prompt.
type PromptMode string

const (
	// PromptModeRawCommand asks the client to return one terminal command.
	PromptModeRawCommand PromptMode = "raw_command"
)

// Client completes one AI-controlled turn.
type Client interface {
	CompleteTurn(context.Context, *TurnPrompt) (TurnResponse, error)
}

// ClientFunc adapts a function into a Client.
type ClientFunc func(context.Context, *TurnPrompt) (TurnResponse, error)

// CompleteTurn calls fn.
func (fn ClientFunc) CompleteTurn(ctx context.Context, prompt *TurnPrompt) (TurnResponse, error) {
	return fn(ctx, prompt)
}

// TurnPrompt is the provider-neutral state sent to an AI client.
type TurnPrompt struct {
	Mode           PromptMode
	SeriesID       app.SeriesID
	MatchID        app.MatchID
	MatchNumber    int
	TurnNumber     int
	Attempt        int
	Seat           domain.Seat
	CanConcede     bool
	View           app.SeatView
	Hand           []domain.Card
	LegalActions   []ActionOption
	PreviousErrors []string
}

// ActionOption is one legal action plus its stable text command hint.
type ActionOption struct {
	ID      int
	Command string
	Action  domain.Action
}

// TurnResponse is the provider-neutral AI answer.
type TurnResponse struct {
	TextCommand string
}

func buildRawCommandPrompt(turn *app.TurnContext, attempt int, previousErrors []string) TurnPrompt {
	return TurnPrompt{
		Mode:           PromptModeRawCommand,
		SeriesID:       turn.SeriesID,
		MatchID:        turn.MatchID,
		MatchNumber:    turn.MatchNumber,
		TurnNumber:     turn.TurnNumber,
		Attempt:        attempt,
		Seat:           turn.Seat,
		CanConcede:     turn.CanConcede,
		View:           cloneSeatView(&turn.SeatView),
		Hand:           slices.Clone(turn.Hand),
		LegalActions:   buildActionOptions(turn.LegalActions),
		PreviousErrors: slices.Clone(previousErrors),
	}
}

func cloneTurnPrompt(prompt *TurnPrompt) TurnPrompt {
	if prompt == nil {
		return TurnPrompt{}
	}
	cloned := *prompt
	cloned.View = cloneSeatView(&prompt.View)
	cloned.Hand = slices.Clone(prompt.Hand)
	cloned.LegalActions = slices.Clone(prompt.LegalActions)
	cloned.PreviousErrors = slices.Clone(prompt.PreviousErrors)
	return cloned
}

func buildActionOptions(actions []domain.Action) []ActionOption {
	options := make([]ActionOption, 0, len(actions))
	for index, action := range actions {
		options = append(options, ActionOption{
			ID:      index + 1,
			Command: textcmd.FormatActionCommand(action),
			Action:  action,
		})
	}
	return options
}

func cloneSeatView(view *app.SeatView) app.SeatView {
	if view == nil {
		return app.SeatView{}
	}
	return app.SeatView{
		Seat:               view.Seat,
		Phase:              view.Phase,
		Attacker:           view.Attacker,
		Defender:           view.Defender,
		TrumpSuit:          view.TrumpSuit,
		TrumpIndicator:     view.TrumpIndicator,
		Table:              slices.Clone(view.Table),
		HandSizes:          slices.Clone(view.HandSizes),
		StockCount:         view.StockCount,
		DiscardCount:       view.DiscardCount,
		SuccessfulDefenses: view.SuccessfulDefenses,
		Winner:             view.Winner,
		Loser:              view.Loser,
	}
}
