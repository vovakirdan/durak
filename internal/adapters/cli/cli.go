// Package cli contains the local command-line adapter for the first playable
// Durak interface.
package cli

import (
	"context"
	"io"

	"github.com/vovakirdan/durak/internal/adapters/bot"
	"github.com/vovakirdan/durak/internal/app"
	"github.com/vovakirdan/durak/internal/domain"
)

const (
	humanSeat = domain.Seat(0)
	botSeat   = domain.Seat(1)
)

// Run starts the local CLI adapter.
func Run(ctx context.Context, in io.Reader, out io.Writer) error {
	session, _, err := app.NewDealtSession(2, domain.DefaultRuleProfile(), domain.DealOptions{})
	if err != nil {
		return err
	}

	game := newGame(session, bot.NewSimpleStrategy(), in, out)
	return game.run(ctx)
}
