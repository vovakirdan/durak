package main

import (
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
