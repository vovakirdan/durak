# durak

Terminal-first Durak game, starting with a local CLI MVP and designed to grow
into a Bubble Tea TUI and Wish-hosted SSH multiplayer daemon.

## Status

Minimal offline CLI is playable against a deterministic bot. The current CLI is
plain text and intentionally small; it is a stepping stone before a richer TUI.

Implemented core pieces:

- 36-card deck, initial deal, trump selection, and first-attacker selection.
- Two-player match state machine with attack, defense, throw-in, transfer, take,
  refill, and match completion.
- Application session layer, in-memory event recorder, and a simple bot strategy.
- CLI commands by action number or short commands, with consecutive matches in
  one local series.

CLI commands:

- `number` selects one of the listed legal actions.
- `a <card>` attacks with a card.
- `d [attack#] <card>` defends an attack.
- `throw <card>` throws in a legal card.
- `tr <card>` transfers an attack with a same-rank card.
- `take` takes cards.
- `done` finishes a defense or pickup.
- `concede` gives up the current match.
- `help` prints commands.
- `quit` exits.

After a result, press Enter or type `next` to start another match. In a series,
the next match starts before the previous match loser.

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

Append public match events to a local JSONL log with:

```sh
go run ./cmd/durak -seed 42 -event-log .cache/events.jsonl -match-id demo-1
```

`-match-id` is optional when `-event-log` is set; the CLI generates one if it is
omitted. In consecutive matches, the first match uses the base id and later
matches append `-2`, `-3`, and so on.

Run a headless simple-bot arena smoke with:

```sh
go run ./cmd/durak arena -matches 100 -seed 42 -max-actions 500
```

Arena mode is a development tool for match stability checks. It currently runs
`simple` vs `simple` through the application headless runner and can write public
events with `-event-log` and `-match-id`.

The Makefile keeps Go build caches under `.cache/` so commands work in
restricted workspaces without writing to the user-level Go cache.

## Planning Docs

- [PRD](docs/2026-06-10-durak-prd.md)
- [Stack](docs/STACK.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Project Rules](docs/PROJECT_RULES.md)
