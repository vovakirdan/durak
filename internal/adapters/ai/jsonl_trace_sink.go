package ai

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
)

const rawCommandTraceSchemaVersion = 1

var errMissingAITraceLogPath = errors.New("missing ai trace log path")

// JSONLTraceSink appends private raw AI command attempts to a JSONL file.
type JSONLTraceSink struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	err     error
}

// NewJSONLTraceSink creates an append-only private trace sink.
func NewJSONLTraceSink(path string) (*JSONLTraceSink, error) {
	if path == "" {
		return nil, errMissingAITraceLogPath
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create ai trace log directory: %w", err)
		}
	}
	// #nosec G304 -- the trace path is an explicit local CLI output path chosen by the user.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open ai trace log: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		closeErr := file.Close()
		return nil, fmt.Errorf("secure ai trace log: %w", errors.Join(err, closeErr))
	}
	return &JSONLTraceSink{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// RecordRawCommandTrace appends one trace record and stores the first write error.
func (s *JSONLTraceSink) RecordRawCommandTrace(trace *RawCommandTrace) {
	if s == nil || trace == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return
	}
	if err := s.encoder.Encode(newRawCommandTraceRecord(trace)); err != nil {
		s.err = fmt.Errorf("write ai trace log: %w", err)
	}
}

// Err returns the first write or close error observed by the sink.
func (s *JSONLTraceSink) Err() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close flushes and closes the trace file.
func (s *JSONLTraceSink) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return s.err
	}
	if err := s.file.Close(); err != nil && s.err == nil {
		s.err = fmt.Errorf("close ai trace log: %w", err)
	}
	s.file = nil
	return s.err
}

type rawCommandTraceRecord struct {
	SchemaVersion int                      `json:"schema_version"`
	RecordedAt    time.Time                `json:"recorded_at"`
	StartedAt     time.Time                `json:"started_at"`
	DurationMS    int64                    `json:"duration_ms"`
	Client        ClientInfo               `json:"client"`
	SeriesID      string                   `json:"series_id,omitempty"`
	MatchID       string                   `json:"match_id,omitempty"`
	MatchNumber   int                      `json:"match_number"`
	TurnNumber    int                      `json:"turn_number"`
	Attempt       int                      `json:"attempt"`
	Seat          int                      `json:"seat"`
	Mode          PromptMode               `json:"mode"`
	CanConcede    bool                     `json:"can_concede"`
	Prompt        subprocessPrompt         `json:"prompt"`
	RawCommand    string                   `json:"raw_command"`
	Usage         *TokenUsage              `json:"usage,omitempty"`
	CommandKind   string                   `json:"command_kind,omitempty"`
	Decision      *rawCommandTraceDecision `json:"decision,omitempty"`
	Err           string                   `json:"error,omitempty"`
}

type rawCommandTraceDecision struct {
	Kind        string `json:"kind"`
	Command     string `json:"command,omitempty"`
	ActionKind  string `json:"action_kind,omitempty"`
	Seat        int    `json:"seat,omitempty"`
	Card        string `json:"card,omitempty"`
	AttackIndex int    `json:"attack_index,omitempty"`
}

func newRawCommandTraceRecord(trace *RawCommandTrace) rawCommandTraceRecord {
	prompt := cloneTurnPrompt(&trace.Prompt)
	return rawCommandTraceRecord{
		SchemaVersion: rawCommandTraceSchemaVersion,
		RecordedAt:    time.Now().UTC(),
		StartedAt:     trace.StartedAt.UTC(),
		DurationMS:    trace.Duration.Milliseconds(),
		Client:        trace.Client,
		SeriesID:      string(prompt.SeriesID),
		MatchID:       string(prompt.MatchID),
		MatchNumber:   prompt.MatchNumber,
		TurnNumber:    prompt.TurnNumber,
		Attempt:       prompt.Attempt,
		Seat:          int(prompt.Seat),
		Mode:          prompt.Mode,
		CanConcede:    prompt.CanConcede,
		Prompt:        newSubprocessPrompt(&prompt),
		RawCommand:    trace.RawCommand,
		Usage:         traceUsage(trace.Usage),
		CommandKind:   traceCommandKind(trace),
		Decision:      traceDecision(trace.Decision),
		Err:           trace.Err,
	}
}

func traceUsage(usage TokenUsage) *TokenUsage {
	if usage.PromptTokens == 0 && usage.CompletionTokens == 0 && usage.TotalTokens == 0 {
		return nil
	}
	cloned := usage
	return &cloned
}

func traceCommandKind(trace *RawCommandTrace) string {
	if trace.Decision.Kind == app.PlayerDecisionUnknown && trace.CommandKind == textcmd.KindAction {
		return ""
	}
	return commandKindName(trace.CommandKind)
}

func commandKindName(kind textcmd.Kind) string {
	switch kind {
	case textcmd.KindAction:
		return "action"
	case textcmd.KindConcede:
		return "concede"
	case textcmd.KindHelp:
		return "help"
	case textcmd.KindQuit:
		return "quit"
	default:
		return "unknown"
	}
}

func traceDecision(decision app.PlayerDecision) *rawCommandTraceDecision {
	switch decision.Kind {
	case app.PlayerDecisionAction:
		action := decision.Action
		return &rawCommandTraceDecision{
			Kind:        "action",
			Command:     textcmd.FormatActionCommand(action),
			ActionKind:  actionKindName(action.Kind),
			Seat:        int(action.Seat),
			Card:        cardCode(action.Card),
			AttackIndex: action.AttackIndex,
		}
	case app.PlayerDecisionConcede:
		return &rawCommandTraceDecision{
			Kind:    "concede",
			Command: "concede",
		}
	default:
		return nil
	}
}
