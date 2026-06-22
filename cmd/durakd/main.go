package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/vovakirdan/durak/internal/app/server"
)

func main() {
	if err := run(context.Background(), os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "durakd: %v\n", err)
		os.Exit(1)
	}
}

func run(_ context.Context, args []string, out io.Writer) error {
	if len(args) > 1 {
		return fmt.Errorf("unknown argument %q", args[1])
	}
	if len(args) == 1 && args[0] != "status" {
		return fmt.Errorf("unknown command %q", args[0])
	}
	registry := server.NewRegistry()
	_, err := fmt.Fprintf(out, "durakd status: ok tables=%d\n", registry.TableCount())
	return err
}
