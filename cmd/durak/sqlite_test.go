package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunArenaWritesSQLiteHistoryAndReplay(t *testing.T) {
	var arenaOut bytes.Buffer
	var arenaErr bytes.Buffer
	dbPath := filepath.Join(t.TempDir(), "arena.db")

	err := run(context.Background(), []string{
		"arena",
		"-matches", "2",
		"-seed", "42",
		"-db", dbPath,
		"-match-id", "sqlite-arena",
	}, strings.NewReader(""), &arenaOut, &arenaErr)
	if err != nil {
		t.Fatalf("run arena returned error: %v; stderr=%q", err, arenaErr.String())
	}

	var historyOut bytes.Buffer
	var historyErr bytes.Buffer
	err = run(context.Background(), []string{
		"history",
		"-db", dbPath,
	}, strings.NewReader(""), &historyOut, &historyErr)
	if err != nil {
		t.Fatalf("run history returned error: %v; stderr=%q", err, historyErr.String())
	}
	history := historyOut.String()
	for _, want := range []string{
		"History:",
		"match=sqlite-arena ",
		"match=sqlite-arena-2 ",
		"status=complete",
		"rule=default",
	} {
		if !strings.Contains(history, want) {
			t.Fatalf("history = %q, want %q", history, want)
		}
	}

	var replayOut bytes.Buffer
	var replayErr bytes.Buffer
	err = run(context.Background(), []string{
		"replay",
		"-db", dbPath,
		"-match-id", "sqlite-arena",
	}, strings.NewReader(""), &replayOut, &replayErr)
	if err != nil {
		t.Fatalf("run replay returned error: %v; stderr=%q", err, replayErr.String())
	}
	replay := replayOut.String()
	for _, want := range []string{
		"Replay: match=sqlite-arena ",
		"phase=complete",
		"players=2",
		"result=",
	} {
		if !strings.Contains(replay, want) {
			t.Fatalf("replay = %q, want %q", replay, want)
		}
	}

	var analyzeOut bytes.Buffer
	var analyzeErr bytes.Buffer
	err = run(context.Background(), []string{
		"analyze",
		"-db", dbPath,
		"-match-id", "sqlite-arena",
		"-limit", "3",
	}, strings.NewReader(""), &analyzeOut, &analyzeErr)
	if err != nil {
		t.Fatalf("run analyze returned error: %v; stderr=%q", err, analyzeErr.String())
	}
	analysis := analyzeOut.String()
	for _, want := range []string{
		"Analysis: match=sqlite-arena ",
		"moves=",
		"avg_loss=",
		"best=",
		"blunder=",
		"Worst moves:",
		"quality=",
		"action=",
	} {
		if !strings.Contains(analysis, want) {
			t.Fatalf("analysis = %q, want %q", analysis, want)
		}
	}
}

func TestRunPlayWritesSQLiteHistory(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	dbPath := filepath.Join(t.TempDir(), "play.db")

	err := run(context.Background(), []string{
		"-seed", "42",
		"-db", dbPath,
		"-match-id", "sqlite-play",
	}, strings.NewReader("concede\n"), &out, &errOut)
	if err != nil {
		t.Fatalf("run play returned error: %v; stderr=%q", err, errOut.String())
	}

	var historyOut bytes.Buffer
	var historyErr bytes.Buffer
	err = run(context.Background(), []string{
		"history",
		"-db", dbPath,
	}, strings.NewReader(""), &historyOut, &historyErr)
	if err != nil {
		t.Fatalf("run history returned error: %v; stderr=%q", err, historyErr.String())
	}
	history := historyOut.String()
	for _, want := range []string{
		"match=sqlite-play ",
		"status=complete",
		"result=winner=",
		"conceded_by=0",
	} {
		if !strings.Contains(history, want) {
			t.Fatalf("history = %q, want %q", history, want)
		}
	}
}
