package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunStartsAndQuits(t *testing.T) {
	var out bytes.Buffer

	if err := Run(context.Background(), strings.NewReader("q\n"), &out); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Durak CLI") {
		t.Fatalf("output = %q, want header", output)
	}
	if !strings.Contains(output, "Bye.") {
		t.Fatalf("output = %q, want quit message", output)
	}
}
