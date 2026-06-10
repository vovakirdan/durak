package main

import (
	"context"
	"fmt"
	"os"

	"github.com/vovakirdan/durak/internal/adapters/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "durak: %v\n", err)
		os.Exit(1)
	}
}
