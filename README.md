# durak

Terminal-first Durak game, starting with a local CLI MVP and designed to grow
into a Bubble Tea TUI and Wish-hosted SSH multiplayer daemon.

## Status

Minimal offline CLI is playable against a deterministic bot. The current CLI is
plain text and intentionally small; it is a stepping stone before a richer TUI.

Implemented core pieces:

- 36-card deck, initial deal, trump selection, and first-attacker selection.
- Two-player match state machine with attack, defense, throw-in, take, refill,
  and match completion.
- Application session layer and a simple bot strategy.
- CLI commands by action number or short commands.

CLI commands:

- `number` selects one of the listed legal actions.
- `a <card>` attacks with a card.
- `d [attack#] <card>` defends an attack.
- `throw <card>` throws in a legal card.
- `take` takes cards.
- `done` finishes a defense or pickup.
- `help` prints commands.
- `quit` exits.

## Development

```sh
make check
make run
make build
```

Run a replayable CLI deal with:

```sh
go run ./cmd/durak -seed 42
```

The Makefile keeps Go build caches under `.cache/` so commands work in
restricted workspaces without writing to the user-level Go cache.

## Planning Docs

- [PRD](docs/2026-06-10-durak-prd.md)
- [Stack](docs/STACK.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Project Rules](docs/PROJECT_RULES.md)
