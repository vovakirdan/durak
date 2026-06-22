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

	options, err := parseSSHOptions([]string{"-addr", "127.0.0.1:0"}, &out, hostKey)
	if err != nil {
		t.Fatalf("parseSSHOptions returned error: %v", err)
	}

	if options.Addr != "127.0.0.1:0" {
		t.Fatalf("Addr = %q, want 127.0.0.1:0", options.Addr)
	}
	if options.HostKeyPath != hostKey {
		t.Fatalf("HostKeyPath = %q, want %q", options.HostKeyPath, hostKey)
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
}
