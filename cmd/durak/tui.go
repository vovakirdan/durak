package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/vovakirdan/durak/internal/adapters/tui"
	"github.com/vovakirdan/durak/internal/app/client"
	"github.com/vovakirdan/durak/internal/domain"
)

func runTUI(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	var seed seedFlag
	var seats int
	var humanSeat int
	aiConfig := newAIFlags()
	botName := normalizePlayerControllerKind("")
	players := defaultArenaPlayers()
	rulesName := defaultRulesPreset
	flags := flag.NewFlagSet("durak tui", flag.ContinueOnError)
	flags.SetOutput(errOut)
	flags.Var(&seed, "seed", "deterministic deal seed for replayable games")
	flags.StringVar(&botName, "bot", botName, "bot controller: "+controllerNames())
	flags.IntVar(&seats, "seats", 2, "number of active seats")
	flags.IntVar(&humanSeat, "human-seat", 0, "local human seat index")
	for seat := range players {
		flags.StringVar(&players[seat], fmt.Sprintf("p%d", seat), "", fmt.Sprintf("seat %d controller override: %s", seat, controllerNames()))
	}
	flags.StringVar(&rulesName, "rules", rulesName, "rule preset: default")
	aiConfig.bind(flags)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	if err := validatePlayerControllerKind(botName); err != nil {
		return err
	}
	config, err := matchConfig(rulesName, seats)
	if err != nil {
		return err
	}
	if humanSeat < 0 || humanSeat >= seats {
		return fmt.Errorf("human-seat must be in range 0..%d", seats-1)
	}
	aiTraceSink, err := aiConfig.openTraceSink()
	if err != nil {
		return err
	}
	controllers, err := playControllers(&playControllerOptions{
		seats:     seats,
		humanSeat: domain.Seat(humanSeat),
		botName:   botName,
		players:   players,
		seed:      seed.value,
		seeded:    seed.set,
		aiConfig:  &aiConfig,
		traceSink: aiTraceSink,
	})
	if err != nil {
		return closeAITraceSink(aiTraceSink, err)
	}
	options := client.LocalGameOptions{
		SeriesID:    "tui-series",
		BaseMatchID: "tui-match",
		PlayerCount: seats,
		HumanSeat:   domain.Seat(humanSeat),
		Config:      config,
		Controllers: controllers,
	}
	if seed.set {
		options.Deal = domain.SeededDealOptions(seed.value)
	}
	game, err := client.NewLocalGame(ctx, &options)
	if err != nil {
		return closeAITraceSink(aiTraceSink, err)
	}
	return closeAITraceSink(aiTraceSink, tui.Run(ctx, in, out, game))
}
