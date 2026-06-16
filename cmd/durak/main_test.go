package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewMatchID(t *testing.T) {
	matchID, err := newMatchID()
	if err != nil {
		t.Fatalf("newMatchID returned error: %v", err)
	}

	id := string(matchID)
	if !strings.HasPrefix(id, "cli-") {
		t.Fatalf("match id = %q, want cli- prefix", id)
	}
	if len(id) != len("cli-")+32 {
		t.Fatalf("match id length = %d, want %d", len(id), len("cli-")+32)
	}
}

func TestRunArenaCompletesMatches(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(t.Context(), []string{
		"arena",
		"-matches", "3",
		"-seed", "42",
		"-max-actions", "800",
		"-p0", "simple",
		"-p1", "random",
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"Arena: seat0=simple seat1=random",
		"Rules: default",
		"Matches: 3",
		"Seed: 42",
		"Max actions/match: 800",
		"Results: seat0=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunArenaCompletesThreeSeatMatches(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(t.Context(), []string{
		"arena",
		"-matches", "3",
		"-seats", "3",
		"-seed", "42",
		"-max-actions", "1000",
		"-p0", "simple",
		"-p1", "random",
		"-p2", "random",
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"Arena: seat0=simple seat1=random seats=3",
		"players=[0:simple,1:random,2:random]",
		"Matches: 3",
		"Results: seat0=",
		"seat2=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunArenaAcceptsRawAIController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(t.Context(), []string{
		"arena",
		"-matches", "3",
		"-seed", "42",
		"-max-actions", "800",
		"-p0", "simple",
		"-p1", "ai-raw-mock",
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"Arena: seat0=simple seat1=ai-raw-mock",
		"Matches: 3",
		"Results: seat0=",
		"Raw AI: attempts=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunArenaWritesAITraceLog(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	path := filepath.Join(t.TempDir(), "ai-trace.jsonl")

	err := run(t.Context(), []string{
		"arena",
		"-matches", "1",
		"-seed", "42",
		"-max-actions", "800",
		"-p0", "simple",
		"-p1", "ai-raw-mock",
		"-ai-trace-log", path,
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	assertAITraceLog(t, path, `"client":{"provider":"mock","model":"noisy-raw-command"`)
}

func TestRunArenaAcceptsRawExecAIController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	script := writeTestExecutable(t, "#!/bin/sh\nprintf '1\\n'\n")

	err := run(t.Context(), []string{
		"arena",
		"-matches", "1",
		"-seed", "42",
		"-max-actions", "800",
		"-p0", "simple",
		"-p1", "ai-raw-exec",
		"-ai-command", script,
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"Arena: seat0=simple seat1=ai-raw-exec",
		"Matches: 1",
		"Raw AI: attempts=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunArenaAcceptsOpenAIController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	server := newOpenAICommandServer(t, "1")

	err := run(t.Context(), []string{
		"arena",
		"-matches", "1",
		"-seed", "42",
		"-max-actions", "800",
		"-p0", "simple",
		"-p1", "ai-openai",
		"-ai-base-url", server.URL + "/v1",
		"-ai-model", "test-model",
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"Arena: seat0=simple seat1=ai-openai",
		"Matches: 1",
		"Raw AI: attempts=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunArenaWritesEventLog(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	path := filepath.Join(t.TempDir(), "arena.jsonl")

	err := run(context.Background(), []string{
		"arena",
		"-matches", "2",
		"-event-log", path,
		"-match-id", "arena-test",
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, errOut.String())
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(data), `"match_id":"arena-test"`) {
		t.Fatalf("event log = %q, want first match id", string(data))
	}
	if !strings.Contains(string(data), `"match_id":"arena-test-2"`) {
		t.Fatalf("event log = %q, want second match id", string(data))
	}
}

func TestRunHistoryReadsEventLog(t *testing.T) {
	var arenaOut bytes.Buffer
	var arenaErr bytes.Buffer
	path := filepath.Join(t.TempDir(), "history.jsonl")

	err := run(context.Background(), []string{
		"arena",
		"-matches", "2",
		"-seed", "42",
		"-event-log", path,
		"-match-id", "history-test",
	}, strings.NewReader(""), &arenaOut, &arenaErr)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, arenaErr.String())
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	err = run(context.Background(), []string{
		"history",
		"-event-log", path,
	}, strings.NewReader(""), &out, &errOut)
	if err != nil {
		t.Fatalf("run history returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"History:",
		"match=history-test ",
		"match=history-test-2 ",
		"status=complete",
		"seats=[0,1]",
		"rule=default",
		"result=winner=",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunHistoryRequiresEventLog(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"history"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run history returned nil error, want missing history source")
	}
	if !strings.Contains(err.Error(), "event-log or db is required") {
		t.Fatalf("error = %v, want history source error", err)
	}
}

func TestRunReplayRequiresSQLiteInputs(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"replay"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run replay returned nil error, want missing db")
	}
	if !strings.Contains(err.Error(), "db is required") {
		t.Fatalf("error = %v, want db error", err)
	}
}

func TestRunArenaRejectsInvalidMatches(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"arena", "-matches", "0"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run arena returned nil error, want invalid matches")
	}
	if !strings.Contains(err.Error(), "matches must be positive") {
		t.Fatalf("error = %v, want positive matches error", err)
	}
}

func TestRunArenaRejectsInvalidSeats(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"arena", "-seats", "7"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run arena returned nil error, want invalid seats")
	}
	if !strings.Contains(err.Error(), "seats must be in range 2..6") {
		t.Fatalf("error = %v, want seats range error", err)
	}
}

func TestRunArenaRejectsUnknownController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"arena", "-p0", "unknown"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run arena returned nil error, want invalid controller")
	}
	if !strings.Contains(err.Error(), `p0: unknown player controller: "unknown"`) {
		t.Fatalf("error = %v, want unknown controller error", err)
	}
}

func TestRunPlayAcceptsBotAndRuleFlags(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{
		"-seed", "42",
		"-bot", "random",
		"-rules", "default",
	}, strings.NewReader("q\n"), &out, &errOut)
	if err != nil {
		t.Fatalf("run play returned error: %v; stderr=%q", err, errOut.String())
	}

	if !strings.Contains(out.String(), "Durak CLI") {
		t.Fatalf("output = %q, want CLI header", out.String())
	}
}

func TestRunPlayAcceptsThreeSeatFlags(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{
		"-seed", "42",
		"-seats", "3",
		"-human-seat", "0",
		"-bot", "simple",
		"-p2", "random",
	}, strings.NewReader("q\n"), &out, &errOut)
	if err != nil {
		t.Fatalf("run play returned error: %v; stderr=%q", err, errOut.String())
	}

	output := out.String()
	for _, want := range []string{
		"Durak CLI",
		"Hands: you(0):6 bot(1):6 bot(2):6",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output = %q, want %q", output, want)
		}
	}
}

func TestRunPlayRejectsInvalidHumanSeat(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"-seats", "3", "-human-seat", "3"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run play returned nil error, want invalid human seat")
	}
	if !strings.Contains(err.Error(), "human-seat must be in range 0..2") {
		t.Fatalf("error = %v, want human-seat range error", err)
	}
}

func TestRunPlayRejectsUnknownSeatController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"-seats", "3", "-p2", "unknown"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run play returned nil error, want invalid seat controller")
	}
	if !strings.Contains(err.Error(), `p2: unknown player controller: "unknown"`) {
		t.Fatalf("error = %v, want unknown p2 controller error", err)
	}
}

func TestRunPlayAcceptsRawExecAIController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	script := writeTestExecutable(t, "#!/bin/sh\nprintf '1\\n'\n")

	err := run(t.Context(), []string{
		"-seed", "42",
		"-bot", "ai-raw-exec",
		"-ai-command", script,
	}, strings.NewReader("q\n"), &out, &errOut)
	if err != nil {
		t.Fatalf("run play returned error: %v; stderr=%q", err, errOut.String())
	}
	if !strings.Contains(out.String(), "Durak CLI") {
		t.Fatalf("output = %q, want CLI header", out.String())
	}
}

func TestRunPlayWritesAITraceLog(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	path := filepath.Join(t.TempDir(), "play-ai-trace.jsonl")

	err := run(t.Context(), []string{
		"-seed", "42",
		"-bot", "ai-raw-mock",
		"-ai-trace-log", path,
	}, strings.NewReader("1\nq\n"), &out, &errOut)
	if err != nil {
		t.Fatalf("run play returned error: %v; stderr=%q", err, errOut.String())
	}

	assertAITraceLog(t, path, `"client":{"provider":"mock","model":"noisy-raw-command"`)
}

func TestRunPlayAcceptsOpenAIController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	server := newOpenAICommandServer(t, "1")

	err := run(t.Context(), []string{
		"-seed", "42",
		"-bot", "ai-openai",
		"-ai-base-url", server.URL + "/v1",
		"-ai-model", "test-model",
	}, strings.NewReader("q\n"), &out, &errOut)
	if err != nil {
		t.Fatalf("run play returned error: %v; stderr=%q", err, errOut.String())
	}
	if !strings.Contains(out.String(), "Durak CLI") {
		t.Fatalf("output = %q, want CLI header", out.String())
	}
}

func TestRunPlayRequiresRawExecAICommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(t.Context(), []string{"-bot", "ai-raw-exec"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run play returned nil error, want missing AI command")
	}
	if !strings.Contains(err.Error(), "ai-raw-exec requires -ai-command") {
		t.Fatalf("error = %v, want missing AI command error", err)
	}
}

func TestRunPlayRequiresOpenAIModel(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(t.Context(), []string{
		"-bot", "ai-openai",
		"-ai-base-url", "http://127.0.0.1:11434/v1",
	}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run play returned nil error, want missing AI model")
	}
	if !strings.Contains(err.Error(), "missing openai-compatible model") {
		t.Fatalf("error = %v, want missing model error", err)
	}
}

func TestRunPlayRejectsUnknownBot(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"-bot", "unknown"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run play returned nil error, want invalid bot")
	}
	if !strings.Contains(err.Error(), `unknown player controller: "unknown"`) {
		t.Fatalf("error = %v, want unknown bot error", err)
	}
}

func TestRunRejectsUnknownRules(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"-rules", "custom"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run play returned nil error, want invalid rules")
	}
	if !strings.Contains(err.Error(), `unknown rules preset "custom"`) {
		t.Fatalf("error = %v, want unknown rules error", err)
	}
}

func writeTestExecutable(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "command.sh")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}

func assertAITraceLog(t *testing.T, path string, wantClient string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	log := string(data)
	for _, want := range []string{
		`"schema_version":1`,
		wantClient,
		`"prompt":`,
		`"raw_command":`,
	} {
		if !strings.Contains(log, want) {
			t.Fatalf("trace log = %q, want %q", log, want)
		}
	}
}

func newOpenAICommandServer(t *testing.T, command string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1,
			"model":"test-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"` + command + `"},"finish_reason":"stop"}]
		}`))
	}))
	t.Cleanup(server.Close)
	return server
}
