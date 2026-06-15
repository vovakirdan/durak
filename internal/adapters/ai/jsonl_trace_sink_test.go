package ai_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/ai"
	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
)

func TestJSONLTraceSinkWritesPrivateRawCommandTrace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "ai-trace.jsonl")
	sink, err := ai.NewJSONLTraceSink(path)
	if err != nil {
		t.Fatalf("NewJSONLTraceSink returned error: %v", err)
	}

	prompt := subprocessTurnPrompt()
	sink.RecordRawCommandTrace(&ai.RawCommandTrace{
		Prompt:      *prompt,
		Client:      ai.ClientInfo{Provider: "openai-compatible", Model: "test-model", BaseURL: "http://127.0.0.1:11434/v1"},
		StartedAt:   time.Unix(123, 0),
		Duration:    1500 * time.Millisecond,
		RawCommand:  "attack 6C",
		Usage:       ai.TokenUsage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
		CommandKind: textcmd.KindAction,
		Decision:    app.ActionDecision(prompt.LegalActions[0].Action),
	})
	if closeErr := sink.Close(); closeErr != nil {
		t.Fatalf("Close returned error: %v", closeErr)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("trace log mode = %v, want 0600", info.Mode().Perm())
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer file.Close()

	var record aiTraceLogRecord
	if err := json.NewDecoder(file).Decode(&record); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if record.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", record.SchemaVersion)
	}
	if record.Client.Provider != "openai-compatible" || record.Client.Model != "test-model" {
		t.Fatalf("client = %+v, want provider metadata", record.Client)
	}
	if record.Client.BaseURL != "http://127.0.0.1:11434/v1" {
		t.Fatalf("client base_url = %q, want configured base URL", record.Client.BaseURL)
	}
	if record.RawCommand != "attack 6C" || record.CommandKind != "action" {
		t.Fatalf("raw command trace = %+v, want parsed action command", record)
	}
	if record.DurationMS != 1500 {
		t.Fatalf("duration_ms = %d, want 1500", record.DurationMS)
	}
	if record.Usage == nil || record.Usage.TotalTokens != 12 {
		t.Fatalf("usage = %+v, want token usage", record.Usage)
	}
	if len(record.Prompt.Hand) != 1 || record.Prompt.Hand[0] != "6C" {
		t.Fatalf("prompt hand = %v, want private hand in trace", record.Prompt.Hand)
	}
	if record.Decision == nil || record.Decision.Command != "attack 6C" {
		t.Fatalf("decision = %+v, want action decision", record.Decision)
	}
}

type aiTraceLogRecord struct {
	SchemaVersion int           `json:"schema_version"`
	DurationMS    int64         `json:"duration_ms"`
	Client        ai.ClientInfo `json:"client"`
	Prompt        struct {
		Hand []string `json:"hand"`
	} `json:"prompt"`
	RawCommand  string         `json:"raw_command"`
	Usage       *ai.TokenUsage `json:"usage"`
	CommandKind string         `json:"command_kind"`
	Decision    *struct {
		Command string `json:"command"`
	} `json:"decision"`
}
