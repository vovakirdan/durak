// Package cli contains the local command-line adapter for the first playable
// Durak interface.
package cli

import (
	"context"
	"fmt"
	"io"
)

// Run starts the local CLI adapter.
func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	_ = ctx
	_ = in
	_, err := fmt.Fprintln(out, "Durak CLI bootstrap: game loop is not implemented yet.")
	return err
}
