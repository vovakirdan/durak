package ai

import "context"

// NoisyRawCommandClient is a deterministic local client for parser stress tests.
type NoisyRawCommandClient struct{}

// CompleteTurn returns occasional invalid first-attempt commands, then a legal hint.
func (NoisyRawCommandClient) CompleteTurn(ctx context.Context, prompt *TurnPrompt) (TurnResponse, error) {
	if err := ctx.Err(); err != nil {
		return TurnResponse{}, err
	}
	if prompt == nil {
		return TurnResponse{TextCommand: "help"}, nil
	}
	if prompt.Attempt == 1 {
		switch {
		case prompt.TurnNumber%11 == 0:
			return TurnResponse{TextCommand: "attack ZZ"}, nil
		case prompt.TurnNumber%7 == 0:
			return TurnResponse{TextCommand: "please play the best card"}, nil
		}
	}
	if len(prompt.LegalActions) == 0 {
		return TurnResponse{TextCommand: "help"}, nil
	}
	return TurnResponse{TextCommand: prompt.LegalActions[0].Command}, nil
}
