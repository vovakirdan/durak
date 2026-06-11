package main

import (
	"bytes"
	"context"
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

	err := run(context.Background(), []string{
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

func TestRunArenaRejectsUnknownController(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := run(context.Background(), []string{"arena", "-p0", "unknown"}, strings.NewReader(""), &out, &errOut)
	if err == nil {
		t.Fatal("run arena returned nil error, want invalid controller")
	}
	if !strings.Contains(err.Error(), `p0: unknown arena controller "unknown"`) {
		t.Fatalf("error = %v, want unknown controller error", err)
	}
}
