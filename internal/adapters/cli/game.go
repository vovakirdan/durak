package cli

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

type game struct {
	session  *app.Session
	bot      app.Strategy
	scanner  *bufio.Scanner
	out      *output
	renderer renderer
}

func newGame(session *app.Session, bot app.Strategy, in io.Reader, out io.Writer) *game {
	return &game{
		session:  session,
		bot:      bot,
		scanner:  bufio.NewScanner(in),
		out:      newOutput(out),
		renderer: renderer{},
	}
}

func (g *game) run(ctx context.Context) error {
	g.out.println("Durak CLI")
	g.renderer.writeHelp(g.out)
	if err := g.out.result(); err != nil {
		return err
	}

	for {
		if err := g.runBotTurns(ctx); err != nil {
			return err
		}

		decision := g.session.DecisionContext(humanSeat)
		g.renderer.writeState(g.out, &decision)
		if decision.Phase == domain.MatchPhaseComplete {
			return g.out.result()
		}

		g.out.print("> ")
		if err := g.out.result(); err != nil {
			return err
		}
		if !g.scanner.Scan() {
			return g.scanner.Err()
		}

		command, err := parseCommand(g.scanner.Text(), &decision)
		if err != nil {
			g.out.printf("Invalid command: %v\n", err)
			if err := g.out.result(); err != nil {
				return err
			}
			continue
		}

		switch command.kind {
		case commandQuit:
			g.out.println("Bye.")
			return g.out.result()
		case commandHelp:
			g.renderer.writeHelp(g.out)
			if err := g.out.result(); err != nil {
				return err
			}
		case commandAction:
			if err := g.session.ApplyAction(ctx, command.action); err != nil {
				g.out.printf("Illegal action: %v\n", err)
				if err := g.out.result(); err != nil {
					return err
				}
			}
		}
	}
}

func (g *game) runBotTurns(ctx context.Context) error {
	for {
		view := g.session.ViewForSeat(humanSeat)
		if activeSeat(&view) != botSeat {
			return nil
		}
		action, err := g.session.ApplyStrategy(ctx, botSeat, g.bot)
		if err != nil {
			return err
		}
		g.out.printf("Bot: %s\n", formatAction(action))
		if err := g.out.result(); err != nil {
			return err
		}
	}
}

func activeSeat(view *app.SeatView) domain.Seat {
	if view == nil {
		return domain.NoSeat
	}
	switch view.Phase {
	case domain.MatchPhaseAttack, domain.MatchPhaseThrowIn, domain.MatchPhaseTaking:
		return view.Attacker
	case domain.MatchPhaseDefense:
		return view.Defender
	case domain.MatchPhaseComplete:
		return domain.NoSeat
	default:
		return domain.NoSeat
	}
}

func isQuit(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "q", "quit", "exit":
		return true
	default:
		return false
	}
}

func isHelp(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "h", "help", "?":
		return true
	default:
		return false
	}
}

func commandError(message string) error {
	return errors.New(message)
}
