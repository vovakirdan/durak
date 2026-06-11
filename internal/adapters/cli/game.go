package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

type game struct {
	session   *app.Session
	bot       app.Strategy
	humanSeat domain.Seat
	botSeat   domain.Seat
	scanner   *bufio.Scanner
	out       *output
	renderer  renderer
	startNext nextMatchFunc
	complete  completeMatchFunc
	matchNo   int
}

type gameOptions struct {
	humanSeat domain.Seat
	botSeat   domain.Seat
	startNext nextMatchFunc
	complete  completeMatchFunc
}

type nextMatchFunc func(context.Context) (*app.Session, error)
type completeMatchFunc func(*app.Session) error

func newGame(session *app.Session, bot app.Strategy, in io.Reader, out io.Writer, options gameOptions) *game {
	return &game{
		session:   session,
		bot:       bot,
		humanSeat: options.humanSeat,
		botSeat:   options.botSeat,
		scanner:   bufio.NewScanner(in),
		out:       newOutput(out),
		renderer:  newRenderer(options.humanSeat, options.botSeat),
		startNext: options.startNext,
		complete:  options.complete,
		matchNo:   1,
	}
}

func (g *game) run(ctx context.Context) error {
	g.out.println("Durak CLI")
	g.renderer.writeHelp(g.out)
	g.writeMatchBanner()
	if err := g.out.result(); err != nil {
		return err
	}

	for {
		if err := g.runBotTurns(ctx); err != nil {
			return err
		}

		decision := g.session.DecisionContext(g.humanSeat)
		g.renderer.writeState(g.out, &decision)
		if decision.Phase == domain.MatchPhaseComplete {
			next, err := g.promptNextMatch(ctx)
			if err != nil || !next {
				return err
			}
			continue
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
		case commandConcede:
			if err := g.session.Concede(ctx, g.humanSeat); err != nil {
				g.out.printf("Could not concede: %v\n", err)
				if err := g.out.result(); err != nil {
					return err
				}
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

func (g *game) writeMatchBanner() {
	g.out.println()
	g.out.printf("Match #%d\n", g.matchNo)
}

func (g *game) promptNextMatch(ctx context.Context) (bool, error) {
	if err := g.out.result(); err != nil {
		return false, err
	}
	if g.complete != nil {
		if err := g.complete(g.session); err != nil {
			return false, err
		}
	}
	if g.startNext == nil {
		return false, nil
	}

	for {
		g.out.print("Next match? [Enter/next or quit] ")
		if err := g.out.result(); err != nil {
			return false, err
		}
		if !g.scanner.Scan() {
			if err := g.scanner.Err(); err != nil {
				return false, err
			}
			return false, g.out.result()
		}

		input := strings.ToLower(strings.TrimSpace(g.scanner.Text()))
		switch {
		case input == "" || input == "next":
			session, err := g.startNext(ctx)
			if err != nil {
				return false, nextMatchError(g.matchNo+1, err)
			}
			g.session = session
			g.matchNo++
			g.out.printf("Starting match #%d.\n", g.matchNo)
			g.writeMatchBanner()
			return true, g.out.result()
		case isQuit(input):
			g.out.println("Bye.")
			return false, g.out.result()
		case isHelp(input):
			g.out.println("After a result, press Enter or type next to start another match; type quit to exit.")
			g.renderer.writeHelp(g.out)
			if err := g.out.result(); err != nil {
				return false, err
			}
		default:
			g.out.printf("Invalid command: %v\n", commandError("expected next or quit"))
			if err := g.out.result(); err != nil {
				return false, err
			}
		}
	}
}

func (g *game) runBotTurns(ctx context.Context) error {
	for {
		view := g.session.ViewForSeat(g.humanSeat)
		if activeSeat(&view) != g.botSeat {
			return nil
		}
		action, err := g.session.ApplyStrategy(ctx, g.botSeat, g.bot)
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

func isConcede(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "concede", "surrender", "ff":
		return true
	default:
		return false
	}
}

func commandError(message string) error {
	return errors.New(message)
}

func nextMatchError(matchNo int, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("start match %d: %w", matchNo, err)
}
