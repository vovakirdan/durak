package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	sshadapter "github.com/vovakirdan/durak/internal/adapters/ssh"
)

func TestRunStatus(t *testing.T) {
	var out bytes.Buffer

	if err := run(context.Background(), []string{"status"}, &out); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if got := out.String(); !strings.Contains(got, "durakd status: ok tables=0") {
		t.Fatalf("output = %q, want status", got)
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	var out bytes.Buffer

	err := run(context.Background(), []string{"serve"}, &out)
	if err == nil {
		t.Fatal("run returned nil error, want unknown command")
	}
}

func TestParseSSHOptions(t *testing.T) {
	hostKey := filepath.Join(t.TempDir(), "host_ed25519")
	var out bytes.Buffer

	options, err := parseSSHOptions([]string{
		"-addr", "127.0.0.1:0",
		"-bot", "random",
		"-seed", "42",
	}, &out, hostKey)
	if err != nil {
		t.Fatalf("parseSSHOptions returned error: %v", err)
	}

	if options.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q, want 127.0.0.1:0", options.Addr)
	}
	if options.HostKeyPath != hostKey {
		t.Fatalf("HostKeyPath = %q, want %q", options.HostKeyPath, hostKey)
	}
	if options.Game.Bot != "random" {
		t.Fatalf("Bot = %q, want random", options.Game.Bot)
	}
	if options.Game.Seed != 42 || !options.Game.Seeded {
		t.Fatalf("Game seed = %d seeded=%v, want 42 true", options.Game.Seed, options.Game.Seeded)
	}
}

func TestParseSSHOptionsDefaults(t *testing.T) {
	hostKey := filepath.Join(t.TempDir(), "host_ed25519")
	var out bytes.Buffer

	options, err := parseSSHOptions(nil, &out, hostKey)
	if err != nil {
		t.Fatalf("parseSSHOptions returned error: %v", err)
	}

	if options.Addr != sshadapter.DefaultAddr {
		t.Fatalf("Addr = %q, want %q", options.Addr, sshadapter.DefaultAddr)
	}
	if options.HostKeyPath != hostKey {
		t.Fatalf("HostKeyPath = %q, want %q", options.HostKeyPath, hostKey)
	}
	if options.Game.Bot != "simple" {
		t.Fatalf("Bot = %q, want simple", options.Game.Bot)
	}
	if options.Game.Seeded {
		t.Fatal("Seeded = true, want false by default")
	}
}

func TestParseSSHOptionsRejectsUnknownBot(t *testing.T) {
	hostKey := filepath.Join(t.TempDir(), "host_ed25519")
	var out bytes.Buffer

	_, err := parseSSHOptions([]string{"-bot", "missing"}, &out, hostKey)
	if err == nil {
		t.Fatal("parseSSHOptions returned nil error, want unknown bot")
	}
}
