package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	sshadapter "github.com/vovakirdan/durak/internal/adapters/ssh"
	"github.com/vovakirdan/durak/internal/app/server"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "durakd: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) == 0 {
		return runStatus(out)
	}
	switch args[0] {
	case "status":
		if len(args) > 1 {
			return fmt.Errorf("unknown argument %q", args[1])
		}
		return runStatus(out)
	case "ssh":
		return runSSH(ctx, args[1:], out)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runStatus(out io.Writer) error {
	registry := server.NewRegistry()
	_, err := fmt.Fprintf(out, "durakd status: ok tables=%d\n", registry.TableCount())
	return err
}

func runSSH(_ context.Context, args []string, out io.Writer) error {
	hostKeyPath, err := defaultHostKeyPath()
	if err != nil {
		return err
	}
	options, err := parseSSHOptions(args, out, hostKeyPath)
	if err != nil {
		return err
	}
	sshServer, err := sshadapter.NewServer(&options)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "durakd ssh: listening on %s\n", sshServer.Addr); err != nil {
		return err
	}
	return sshServer.ListenAndServe()
}

func parseSSHOptions(args []string, out io.Writer, hostKeyPath string) (sshadapter.ServerOptions, error) {
	var seed optionalSeedFlag
	options := sshadapter.ServerOptions{
		Addr:        sshadapter.DefaultAddr,
		HostKeyPath: hostKeyPath,
	}
	flags := flag.NewFlagSet("durakd ssh", flag.ContinueOnError)
	flags.SetOutput(out)
	flags.StringVar(&options.Addr, "addr", options.Addr, "SSH listen address")
	flags.StringVar(&options.Game.Bot, "bot", bot.ControllerSimple, "bot controller: simple, random, heuristic")
	flags.StringVar(&options.HostKeyPath, "host-key", options.HostKeyPath, "SSH host key path")
	flags.StringVar(&options.TableID, "table", "", "in-memory table id shared by SSH sessions")
	flags.Var(&seed, "seed", "deterministic deal and bot seed")
	if err := flags.Parse(args); err != nil {
		return options, err
	}
	if flags.NArg() != 0 {
		return options, fmt.Errorf("unknown argument %q", flags.Arg(0))
	}
	if err := bot.ValidateControllerKind(options.Game.Bot); err != nil {
		return options, err
	}
	options.Game.Seed = seed.value
	options.Game.Seeded = seed.set
	return options, nil
}

func defaultHostKeyPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	dir = filepath.Join(dir, "durak")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	return filepath.Join(dir, "durakd_ed25519"), nil
}

type optionalSeedFlag struct {
	value uint64
	set   bool
}

func (f *optionalSeedFlag) Set(value string) error {
	seed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return err
	}
	f.value = seed
	f.set = true
	return nil
}

func (f *optionalSeedFlag) String() string {
	if f == nil || !f.set {
		return ""
	}
	return strconv.FormatUint(f.value, 10)
}
