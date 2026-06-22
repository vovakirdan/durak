package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/vovakirdan/durak/internal/app/client"
)

// Model owns TUI presentation state. Game mutations stay behind Game.
type Model struct {
	ctx         context.Context
	game        Game
	state       client.State
	actionInput string
	err         error
}

// Game is the minimal client contract a TUI can drive.
type Game interface {
	State() client.State
	SubmitAction(context.Context, string) (client.State, error)
	Advance(context.Context) (client.State, error)
	Concede(context.Context) (client.State, error)
	NextMatch(context.Context) (client.State, error)
}

// NewModel creates a Bubble Tea model and advances controllers to the human turn.
func NewModel(ctx context.Context, game Game) *Model {
	if ctx == nil {
		ctx = context.Background()
	}
	model := &Model{ctx: ctx, game: game}
	if game == nil {
		model.err = client.ErrInvalidLocalGame
		return model
	}
	model.state, model.err = game.Advance(ctx)
	return model
}

// NewErrorModel creates a model that renders a startup error.
func NewErrorModel(err error) *Model {
	return &Model{ctx: context.Background(), err: err}
}

// Run starts the Bubble Tea program.
func Run(ctx context.Context, in io.Reader, out io.Writer, game Game) error {
	options := []tea.ProgramOption{tea.WithInput(in), tea.WithOutput(out)}
	if !isTerminalOutput(out) {
		options = append(options, tea.WithoutRenderer())
	}
	_, err := tea.NewProgram(NewModel(ctx, game), options...).Run()
	return err
}

// Init implements tea.Model.
func (m *Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	input := key.Key().Text
	if len([]rune(input)) > 1 {
		for _, r := range input {
			updated, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
			next, ok := updated.(*Model)
			if !ok {
				return m, cmd
			}
			m = next
			if cmd != nil {
				return m, cmd
			}
		}
		return m, nil
	}
	switch key.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "enter":
		m.submitBufferedAction()
		return m, nil
	case "backspace":
		if m.actionInput != "" {
			m.actionInput = m.actionInput[:len(m.actionInput)-1]
		}
		return m, nil
	case "c":
		m.actionInput = ""
		m.state, m.err = m.game.Concede(m.ctx)
		return m, nil
	case "n":
		m.actionInput = ""
		if m.state.Phase == "complete" {
			m.state, m.err = m.game.NextMatch(m.ctx)
			if m.err == nil {
				m.state, m.err = m.game.Advance(m.ctx)
			}
		}
		return m, nil
	default:
		if input == "" {
			input = key.String()
		}
		m.handleActionInput(input)
		return m, nil
	}
}

func isTerminalOutput(out io.Writer) bool {
	file, ok := out.(interface {
		Stat() (os.FileInfo, error)
	})
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// View implements tea.Model.
func (m *Model) View() tea.View {
	var b strings.Builder
	if m.err != nil {
		fmt.Fprintf(&b, "Error: %v\n\n", m.err)
	}
	renderState(&b, &m.state)
	if m.actionInput != "" {
		fmt.Fprintf(&b, "\nAction: %s\n", m.actionInput)
	}
	b.WriteString("\nKeys: number action | enter submit | c concede | n next | q quit\n")
	return tea.NewView(b.String())
}

func (m *Model) handleActionInput(input string) {
	if !isDigits(input) {
		m.actionInput = ""
		return
	}
	candidate := m.actionInput + input
	exact, longer := actionIDMatch(&m.state, candidate)
	switch {
	case exact && !longer:
		m.actionInput = ""
		m.submitAction(candidate)
	case exact || longer:
		m.actionInput = candidate
	default:
		m.actionInput = ""
	}
}

func (m *Model) submitBufferedAction() {
	if m.actionInput == "" {
		return
	}
	actionID := m.actionInput
	m.actionInput = ""
	m.submitAction(actionID)
}

func (m *Model) submitAction(actionID string) {
	if m.state.Phase == "complete" || !hasAction(&m.state, actionID) {
		return
	}
	if m.game == nil {
		m.err = client.ErrInvalidLocalGame
		return
	}
	m.state, m.err = m.game.SubmitAction(m.ctx, actionID)
	if m.err != nil {
		return
	}
	m.state, m.err = m.game.Advance(m.ctx)
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func actionIDMatch(state *client.State, actionID string) (exact, longer bool) {
	if state == nil {
		return false, false
	}
	for _, action := range state.LegalActions {
		if action.ID == actionID {
			exact = true
		}
		if strings.HasPrefix(action.ID, actionID) && len(action.ID) > len(actionID) {
			longer = true
		}
	}
	return exact, longer
}

func hasAction(state *client.State, actionID string) bool {
	if state == nil {
		return false
	}
	for _, action := range state.LegalActions {
		if action.ID == actionID {
			return true
		}
	}
	return false
}

func renderState(b *strings.Builder, state *client.State) {
	if state == nil {
		return
	}
	fmt.Fprintf(b, "Durak TUI\n")
	fmt.Fprintf(b, "Match: %s v%d | Phase: %s | You: %d | Attacker: %d | Defender: %d\n",
		state.MatchID, state.Version, state.Phase, state.Seat, state.Attacker, state.Defender)
	fmt.Fprintf(b, "Trump: %s (%s) | Stock: %d | Discard: %d | Hands: %v\n",
		state.TrumpSuit, state.TrumpIndicator.Code, state.StockCount, state.DiscardCount, state.HandSizes)
	if len(state.Table) > 0 {
		b.WriteString("Table:\n")
		for index, pair := range state.Table {
			defense := "--"
			if pair.Defense != nil {
				defense = pair.Defense.Code
			}
			fmt.Fprintf(b, "  %d. %s / %s\n", index+1, pair.Attack.Code, defense)
		}
	}
	b.WriteString("Hand:")
	for index, card := range state.Hand {
		fmt.Fprintf(b, " %d:%s", index+1, card.Code)
	}
	b.WriteString("\nActions:")
	if len(state.LegalActions) == 0 {
		b.WriteString(" none")
	}
	for _, action := range state.LegalActions {
		fmt.Fprintf(b, "\n  %s. %s", action.ID, action.Label)
	}
	if state.Result != "" {
		fmt.Fprintf(b, "\nResult: %s", state.Result)
	}
	b.WriteString("\n")
}
