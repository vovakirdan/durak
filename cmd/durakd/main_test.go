package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
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
