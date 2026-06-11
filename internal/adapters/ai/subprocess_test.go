package ai_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vovakirdan/durak/internal/adapters/ai"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

func TestSubprocessClientSendsPromptAndReadsCommand(t *testing.T) {
	promptPath := filepath.Join(t.TempDir(), "prompt.json")
	script := writeExecutable(t, `#!/bin/sh
cat > "$1"
printf '\nattack 6C\n'
`)
	client := mustSubprocessClient(t, ai.SubprocessClientOptions{
		Command: script,
		Args:    []string{promptPath},
		Timeout: time.Second,
	})

	response, err := client.CompleteTurn(t.Context(), subprocessTurnPrompt())
	if err != nil {
		t.Fatalf("CompleteTurn returned error: %v", err)
	}
	if response.TextCommand != "attack 6C" {
		t.Fatalf("TextCommand = %q, want attack command", response.TextCommand)
	}

	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	prompt := string(data)
	for _, want := range []string{
		`"instruction"`,
		`"mode": "raw_command"`,
		`"hand": [`,
		`"6C"`,
		`"command": "attack 6C"`,
		`"previous_errors": [`,
		`"first error"`,
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt = %s, want %q", prompt, want)
		}
	}
}

func TestSubprocessClientReturnsStderrOnFailure(t *testing.T) {
	script := writeExecutable(t, `#!/bin/sh
echo boom >&2
exit 2
`)
	client := mustSubprocessClient(t, ai.SubprocessClientOptions{Command: script, Timeout: time.Second})

	_, err := client.CompleteTurn(t.Context(), subprocessTurnPrompt())
	if err == nil {
		t.Fatal("CompleteTurn returned nil error, want subprocess failure")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want stderr", err)
	}
}

func TestSubprocessClientRejectsEmptyResponse(t *testing.T) {
	script := writeExecutable(t, "#!/bin/sh\n")
	client := mustSubprocessClient(t, ai.SubprocessClientOptions{Command: script, Timeout: time.Second})

	_, err := client.CompleteTurn(t.Context(), subprocessTurnPrompt())
	if !errors.Is(err, ai.ErrEmptySubprocessResponse) {
		t.Fatalf("error = %v, want ErrEmptySubprocessResponse", err)
	}
}

func TestNewSubprocessClientRequiresCommand(t *testing.T) {
	_, err := ai.NewSubprocessClient(ai.SubprocessClientOptions{})
	if !errors.Is(err, ai.ErrMissingSubprocessCommand) {
		t.Fatalf("error = %v, want ErrMissingSubprocessCommand", err)
	}
}

func mustSubprocessClient(t *testing.T, options ai.SubprocessClientOptions) *ai.SubprocessClient {
	t.Helper()
	client, err := ai.NewSubprocessClient(options)
	if err != nil {
		t.Fatalf("NewSubprocessClient returned error: %v", err)
	}
	return client
}

func subprocessTurnPrompt() *ai.TurnPrompt {
	card := domain.Card{Rank: domain.Six, Suit: domain.Clubs}
	return &ai.TurnPrompt{
		Mode:           ai.PromptModeRawCommand,
		SeriesID:       app.SeriesID("series-test"),
		MatchID:        app.MatchID("match-test"),
		MatchNumber:    2,
		TurnNumber:     3,
		Attempt:        1,
		Seat:           domain.Seat(1),
		CanConcede:     true,
		PreviousErrors: []string{"first error"},
		View: app.SeatView{
			Phase:          domain.MatchPhaseAttack,
			Attacker:       domain.Seat(1),
			Defender:       domain.Seat(0),
			TrumpSuit:      domain.Clubs,
			TrumpIndicator: card,
			Table: []domain.TablePair{
				{Attack: domain.Card{Rank: domain.Seven, Suit: domain.Hearts}},
			},
			HandSizes:  []int{6, 5},
			StockCount: 24,
		},
		Hand: []domain.Card{card},
		LegalActions: []ai.ActionOption{
			{
				ID:      1,
				Command: "attack 6C",
				Action:  domain.Action{Kind: domain.ActionKindAttack, Seat: domain.Seat(1), Card: card},
			},
		},
	}
}

func writeExecutable(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ai-command.sh")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	return path
}
