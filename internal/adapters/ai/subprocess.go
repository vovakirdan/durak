package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/vovakirdan/durak/internal/domain"
)

const defaultSubprocessTimeout = 30 * time.Second

var (
	// ErrMissingSubprocessCommand means the subprocess client has no executable.
	ErrMissingSubprocessCommand = errors.New("missing ai subprocess command")
	// ErrEmptySubprocessResponse means the subprocess produced no raw command.
	ErrEmptySubprocessResponse = errors.New("empty ai subprocess response")
)

// SubprocessClientOptions configures a raw-command AI subprocess.
type SubprocessClientOptions struct {
	Command string
	Args    []string
	Timeout time.Duration
}

// SubprocessClient asks an external process to choose one raw command.
type SubprocessClient struct {
	command string
	args    []string
	timeout time.Duration
}

// NewSubprocessClient creates a client that writes one JSON prompt to stdin and
// reads the first non-empty stdout line as the raw command.
func NewSubprocessClient(options SubprocessClientOptions) (*SubprocessClient, error) {
	if options.Command == "" {
		return nil, ErrMissingSubprocessCommand
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultSubprocessTimeout
	}
	return &SubprocessClient{
		command: options.Command,
		args:    append([]string(nil), options.Args...),
		timeout: timeout,
	}, nil
}

// ClientInfo returns non-secret provider metadata for diagnostics.
func (c *SubprocessClient) ClientInfo() ClientInfo {
	if c == nil {
		return ClientInfo{}
	}
	return ClientInfo{
		Provider: "subprocess",
		Model:    c.command,
	}
}

// CompleteTurn runs the configured subprocess for one AI turn.
func (c *SubprocessClient) CompleteTurn(ctx context.Context, prompt *TurnPrompt) (TurnResponse, error) {
	if c == nil || c.command == "" {
		return TurnResponse{}, ErrMissingSubprocessCommand
	}
	payload, err := json.MarshalIndent(newSubprocessPrompt(prompt), "", "  ")
	if err != nil {
		return TurnResponse{}, fmt.Errorf("marshal ai subprocess prompt: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// #nosec G204 -- ai-raw-exec intentionally runs the user-configured local AI wrapper.
	cmd := exec.CommandContext(runCtx, c.command, c.args...)
	cmd.Stdin = bytes.NewReader(append(payload, '\n'))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return TurnResponse{}, fmt.Errorf("run ai subprocess: %w", runCtx.Err())
		}
		return TurnResponse{}, subprocessRunError(err, stderr.String())
	}

	command := firstNonEmptyLine(stdout.String())
	if command == "" {
		return TurnResponse{}, ErrEmptySubprocessResponse
	}
	return TurnResponse{TextCommand: command}, nil
}

type subprocessPrompt struct {
	Instruction    string                   `json:"instruction"`
	Mode           PromptMode               `json:"mode"`
	SeriesID       string                   `json:"series_id,omitempty"`
	MatchID        string                   `json:"match_id,omitempty"`
	MatchNumber    int                      `json:"match_number"`
	TurnNumber     int                      `json:"turn_number"`
	Attempt        int                      `json:"attempt"`
	Seat           int                      `json:"seat"`
	CanConcede     bool                     `json:"can_concede"`
	View           subprocessView           `json:"view"`
	Hand           []string                 `json:"hand"`
	LegalActions   []subprocessActionOption `json:"legal_actions"`
	PreviousErrors []string                 `json:"previous_errors,omitempty"`
}

type subprocessView struct {
	Phase              string                `json:"phase"`
	Attacker           int                   `json:"attacker"`
	Defender           int                   `json:"defender"`
	TrumpSuit          string                `json:"trump_suit"`
	TrumpIndicator     string                `json:"trump_indicator"`
	Table              []subprocessTablePair `json:"table"`
	HandSizes          []int                 `json:"hand_sizes"`
	StockCount         int                   `json:"stock_count"`
	DiscardCount       int                   `json:"discard_count"`
	SuccessfulDefenses int                   `json:"successful_defenses"`
	Winner             int                   `json:"winner"`
	Loser              int                   `json:"loser"`
}

type subprocessTablePair struct {
	Attack   string `json:"attack"`
	Defense  string `json:"defense,omitempty"`
	Defended bool   `json:"defended"`
}

type subprocessActionOption struct {
	ID          int    `json:"id"`
	Command     string `json:"command"`
	Kind        string `json:"kind"`
	Card        string `json:"card,omitempty"`
	AttackIndex int    `json:"attack_index,omitempty"`
}

func newSubprocessPrompt(prompt *TurnPrompt) subprocessPrompt {
	if prompt == nil {
		return subprocessPrompt{Instruction: rawCommandInstruction()}
	}
	return subprocessPrompt{
		Instruction:    rawCommandInstruction(),
		Mode:           prompt.Mode,
		SeriesID:       string(prompt.SeriesID),
		MatchID:        string(prompt.MatchID),
		MatchNumber:    prompt.MatchNumber,
		TurnNumber:     prompt.TurnNumber,
		Attempt:        prompt.Attempt,
		Seat:           int(prompt.Seat),
		CanConcede:     prompt.CanConcede,
		View:           newSubprocessView(prompt),
		Hand:           formatCards(prompt.Hand),
		LegalActions:   newSubprocessActionOptions(prompt.LegalActions),
		PreviousErrors: append([]string(nil), prompt.PreviousErrors...),
	}
}

func rawCommandInstruction() string {
	return "Return exactly one legal command from legal_actions[].command. Print only that command."
}

func newSubprocessView(prompt *TurnPrompt) subprocessView {
	view := prompt.View
	return subprocessView{
		Phase:              phaseName(view.Phase),
		Attacker:           int(view.Attacker),
		Defender:           int(view.Defender),
		TrumpSuit:          view.TrumpSuit.String(),
		TrumpIndicator:     cardCode(view.TrumpIndicator),
		Table:              newSubprocessTable(view.Table),
		HandSizes:          append([]int(nil), view.HandSizes...),
		StockCount:         view.StockCount,
		DiscardCount:       view.DiscardCount,
		SuccessfulDefenses: view.SuccessfulDefenses,
		Winner:             int(view.Winner),
		Loser:              int(view.Loser),
	}
}

func newSubprocessTable(table []domain.TablePair) []subprocessTablePair {
	result := make([]subprocessTablePair, 0, len(table))
	for _, pair := range table {
		result = append(result, subprocessTablePair{
			Attack:   cardCode(pair.Attack),
			Defense:  cardCode(pair.Defense),
			Defended: pair.Defended,
		})
	}
	return result
}

func newSubprocessActionOptions(actions []ActionOption) []subprocessActionOption {
	options := make([]subprocessActionOption, 0, len(actions))
	for _, action := range actions {
		options = append(options, subprocessActionOption{
			ID:          action.ID,
			Command:     action.Command,
			Kind:        actionKindName(action.Action.Kind),
			Card:        cardCode(action.Action.Card),
			AttackIndex: action.Action.AttackIndex,
		})
	}
	return options
}

func formatCards(cards []domain.Card) []string {
	result := make([]string, 0, len(cards))
	for _, card := range cards {
		result = append(result, cardCode(card))
	}
	return result
}

func cardCode(card domain.Card) string {
	if card.Rank == domain.RankUnknown || card.Suit == domain.SuitUnknown {
		return ""
	}
	return card.String()
}

func phaseName(phase domain.MatchPhase) string {
	switch phase {
	case domain.MatchPhaseAttack:
		return "attack"
	case domain.MatchPhaseDefense:
		return "defense"
	case domain.MatchPhaseThrowIn:
		return "throw_in"
	case domain.MatchPhaseTaking:
		return "taking"
	case domain.MatchPhaseComplete:
		return "complete"
	default:
		return "unknown"
	}
}

func actionKindName(kind domain.ActionKind) string {
	switch kind {
	case domain.ActionKindAttack:
		return "attack"
	case domain.ActionKindDefend:
		return "defend"
	case domain.ActionKindThrowIn:
		return "throw_in"
	case domain.ActionKindPassThrowIn:
		return "pass_throw_in"
	case domain.ActionKindTake:
		return "take"
	case domain.ActionKindFinishDefense:
		return "finish_defense"
	case domain.ActionKindFinishTake:
		return "finish_take"
	case domain.ActionKindTransfer:
		return "transfer"
	default:
		return "unknown"
	}
}

func subprocessRunError(err error, stderr string) error {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return fmt.Errorf("run ai subprocess: %w", err)
	}
	return fmt.Errorf("run ai subprocess: %w: %s", err, stderr)
}

func firstNonEmptyLine(output string) string {
	for line := range strings.Lines(output) {
		if command := strings.TrimSpace(line); command != "" {
			return command
		}
	}
	return ""
}
