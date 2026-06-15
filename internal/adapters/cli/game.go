package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/vovakirdan/durak/internal/adapters/textcmd"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

var errNoPlayableControllerSeat = errors.New("no playable controller seat")

type game struct {
	session     *app.Session
	controllers map[domain.Seat]app.PlayerController
	humanSeat   domain.Seat
	scanner     *bufio.Scanner
	out         *output
	renderer    renderer
	startNext   nextMatchFunc
	complete    completeMatchFunc
	matchNo     int
}

type gameOptions struct {
	humanSeat domain.Seat
	startNext nextMatchFunc
	complete  completeMatchFunc
}

type nextMatchFunc func(context.Context) (*app.Session, error)
type completeMatchFunc func(*app.Session) error

func newGame(session *app.Session, bot app.Strategy, in io.Reader, out io.Writer, options gameOptions) *game {
	return newGameWithController(session, app.StrategyController{Strategy: bot}, in, out, options)
}

func newGameWithController(
	session *app.Session,
	bot app.PlayerController,
	in io.Reader,
	out io.Writer,
	options gameOptions,
) *game {
	return newGameWithControllers(session, map[domain.Seat]app.PlayerController{
		defaultBotSeat: bot,
	}, in, out, options)
}

func newGameWithControllers(
	session *app.Session,
	controllers map[domain.Seat]app.PlayerController,
	in io.Reader,
	out io.Writer,
	options gameOptions,
) *game {
	return &game{
		session:     session,
		controllers: controllers,
		humanSeat:   options.humanSeat,
		scanner:     bufio.NewScanner(in),
		out:         newOutput(out),
		renderer:    newRenderer(options.humanSeat, controllerSeats(controllers)),
		startNext:   options.startNext,
		complete:    options.complete,
		matchNo:     1,
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
		if err := g.runControllerTurns(ctx); err != nil {
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

		command, err := textcmd.Parse(g.scanner.Text(), &decision)
		if err != nil {
			g.out.printf("Invalid command: %v\n", err)
			if err := g.out.result(); err != nil {
				return err
			}
			continue
		}

		switch command.Kind {
		case textcmd.KindQuit:
			g.out.println("Bye.")
			return g.out.result()
		case textcmd.KindHelp:
			g.renderer.writeHelp(g.out)
			if err := g.out.result(); err != nil {
				return err
			}
		case textcmd.KindConcede:
			if err := g.session.Concede(ctx, g.humanSeat); err != nil {
				g.out.printf("Could not concede: %v\n", err)
				if err := g.out.result(); err != nil {
					return err
				}
			}
		case textcmd.KindAction:
			if err := g.session.ApplyAction(ctx, command.Action); err != nil {
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
		case textcmd.IsQuit(input):
			g.out.println("Bye.")
			return false, g.out.result()
		case textcmd.IsHelp(input):
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

func (g *game) runControllerTurns(ctx context.Context) error {
	for {
		view := g.session.ViewForSeat(g.humanSeat)
		if view.Phase == domain.MatchPhaseComplete {
			return nil
		}
		if len(g.session.DecisionContext(g.humanSeat).LegalActions) > 0 {
			return nil
		}
		seat := g.nextControllerSeat(&view)
		if seat == domain.NoSeat {
			return fmt.Errorf("%w: %s", errNoPlayableControllerSeat, formatPhase(view.Phase))
		}
		turn := app.TurnContext{
			CanConcede:      true,
			DecisionContext: g.session.DecisionContext(seat),
		}
		controllerTurn := turn.Clone()
		decision, err := g.controllers[seat].Decide(ctx, &controllerTurn)
		if err != nil {
			return err
		}
		if err := g.session.ApplyPlayerDecision(ctx, seat, &turn, decision); err != nil {
			return err
		}
		g.out.printf("%s: %s\n", g.renderer.formatActor(seat), formatPlayerDecision(decision))
		if err := g.out.result(); err != nil {
			return err
		}
	}
}

func (g *game) nextControllerSeat(view *app.SeatView) domain.Seat {
	if view == nil {
		return domain.NoSeat
	}
	for _, seat := range pollingOrder(view) {
		if seat == g.humanSeat || g.controllers[seat] == nil {
			continue
		}
		if len(g.session.DecisionContext(seat).LegalActions) > 0 {
			return seat
		}
	}
	return domain.NoSeat
}

func formatPlayerDecision(decision app.PlayerDecision) string {
	switch decision.Kind {
	case app.PlayerDecisionAction:
		return formatAction(decision.Action)
	case app.PlayerDecisionConcede:
		return "concede"
	default:
		return "unknown"
	}
}

func pollingOrder(view *app.SeatView) []domain.Seat {
	playerCount := len(view.HandSizes)
	if playerCount == 0 {
		return nil
	}
	start := domain.Seat(0)
	switch view.Phase {
	case domain.MatchPhaseAttack, domain.MatchPhaseThrowIn, domain.MatchPhaseTaking:
		start = view.Attacker
	case domain.MatchPhaseDefense:
		start = view.Defender
	}
	order := make([]domain.Seat, 0, playerCount)
	startIndex := ((int(start) % playerCount) + playerCount) % playerCount
	for offset := range playerCount {
		order = append(order, domain.Seat((startIndex+offset)%playerCount))
	}
	return order
}

func controllerSeats(controllers map[domain.Seat]app.PlayerController) []domain.Seat {
	seats := make([]domain.Seat, 0, len(controllers))
	for seat, controller := range controllers {
		if controller != nil {
			seats = append(seats, seat)
		}
	}
	return seats
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
