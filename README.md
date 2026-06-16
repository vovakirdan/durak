# durak

Terminal-first Durak game, starting with a local CLI MVP and designed to grow
into a Bubble Tea TUI and Wish-hosted SSH multiplayer daemon.

## Status

Minimal offline CLI is playable against a deterministic bot. The current CLI is
plain text and intentionally small; it is a stepping stone before a richer TUI.

Implemented core pieces:

- 36-card deck, initial deal, trump selection, and first-attacker selection.
- Match state machine for 2..6 seats with attack, defense, configurable
  throw-in windows, transfer, take, refill, active-seat skipping, and match
  completion.
- Application session layer, event replay/history projection foundation, and a
  simple controller registry for deterministic, random, and heuristic bots.
- Interactive CLI commands by action number or short commands, with one local
  human seat, controller-driven seats up to 6 players, and consecutive matches
  in one local series.

CLI commands:

- `number` selects one of the listed legal actions.
- `a <card>` attacks with a card.
- `d [attack#] <card>` defends an attack.
- `throw <card>` throws in a legal card.
- `pass` declines the current optional throw-in window.
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
go run ./cmd/durak -seed 42 -bot simple -rules default
```

Run an interactive multi-seat CLI game with one local human and controller
seats:

```sh
go run ./cmd/durak -seed 42 -seats 4 -human-seat 0 \
  -bot simple -p2 random -p3 ai-raw-mock
```

Available player controllers for the opponent are `simple`, `random`,
`heuristic`, `ai-raw-mock`, `ai-raw-exec`, and `ai-openai`. The heuristic bot
uses the first seat-view position evaluator and action ranking layer. The AI
mock is a deterministic local tester that returns raw text commands through the
shared parser.
`ai-openai` calls an OpenAI-compatible `/chat/completions` endpoint directly
through the official Go SDK. `ai-raw-exec` is still available for local wrapper
experiments. In interactive play, `-bot` sets the default controller for every
non-human seat and `-p0` through `-p5` override specific seats. The local human
seat is selected with `-human-seat`; only one local human is supported until the
TUI/SSH table surfaces exist. The only rule preset currently exposed through CLI flags is
`default`; external rule config is also a future milestone.

Run against an OpenAI-compatible endpoint with:

```sh
DURAK_AI_API_KEY=... go run ./cmd/durak \
  -seed 42 \
  -bot ai-openai \
  -ai-model gpt-4o-mini
```

For OpenAI-compatible proxies such as LiteLLM, OpenRouter, vLLM, or Ollama
compatible servers, set the base URL as well:

```sh
DURAK_AI_API_KEY=local go run ./cmd/durak \
  -bot ai-openai \
  -ai-base-url http://127.0.0.1:11434/v1 \
  -ai-model llama3.1
```

AI config can also come from `DURAK_AI_BASE_URL`, `DURAK_AI_API_KEY`, and
`DURAK_AI_MODEL` (`OPENAI_BASE_URL`, `OPENAI_API_KEY`, and `OPENAI_MODEL` are
accepted as fallbacks).

Append private raw AI decision traces to a local JSONL log with:

```sh
DURAK_AI_API_KEY=... go run ./cmd/durak \
  -seed 42 \
  -bot ai-openai \
  -ai-model gpt-4o-mini \
  -ai-trace-log .cache/ai-trace.jsonl
```

The AI trace log contains provider metadata, prompt state, private AI hand,
raw command text, token usage when reported, and parse result. It is created as
`0600` local diagnostic data; do not commit or share it as a public event log.

Append public match events to a local JSONL log with:

```sh
go run ./cmd/durak -seed 42 -event-log .cache/events.jsonl -match-id demo-1
```

`-match-id` is optional when `-event-log` is set; the CLI generates one if it is
omitted. In consecutive matches, the first match uses the base id and later
matches append `-2`, `-3`, and so on.

List matches from a public JSONL log with:

```sh
go run ./cmd/durak history -event-log .cache/events.jsonl
```

Write durable match history to SQLite with:

```sh
go run ./cmd/durak -seed 42 -db .cache/durak.db -match-id demo-1
```

The SQLite store records the canonical internal event stream, public event
projection, config snapshot, match metadata, and completed-match summary in one
transactional batch. Internal events include hidden deal state, so local SQLite
databases are private diagnostic/history artifacts, not public exports.

List completed matches from SQLite with:

```sh
go run ./cmd/durak history -db .cache/durak.db
```

Replay one stored match from SQLite with:

```sh
go run ./cmd/durak replay -db .cache/durak.db -match-id demo-1
```

Analyze stored match decisions with the seat-view evaluator:

```sh
go run ./cmd/durak analyze -db .cache/durak.db -match-id demo-1 -limit 5
```

Run a headless arena smoke with:

```sh
go run ./cmd/durak arena -matches 100 -seed 42 -max-actions 500 -p0 heuristic -p1 simple
```

Print a heuristic move-quality summary for arena decisions with:

```sh
go run ./cmd/durak arena -matches 100 -seed 42 -eval -p0 heuristic -p1 simple
```

Run a multi-seat arena smoke with:

```sh
go run ./cmd/durak arena -matches 100 -seats 4 -seed 42 \
  -p0 simple -p1 random -p2 random -p3 ai-raw-mock
```

Arena mode is a development tool for match stability checks. It runs
controllers through the application headless runner and can write public events
with `-event-log` and durable SQLite history with `-db`/`-match-id`. Available
controllers are `simple`, `random`, `heuristic`, `ai-raw-mock`, `ai-raw-exec`,
and `ai-openai`; `random` chooses uniformly from legal actions and does not
evaluate the position, while `heuristic` ranks legal actions from the visible
seat view without oracle hidden cards. `ai-raw-mock` intentionally exercises raw
command parsing and then retries with legal text commands. Arena supports
`-seats 2..6` and controller flags `-p0` through `-p5`; omitted seats use
`simple`. Arena uses `-rules default` unless another supported preset is
provided later. The optional `-eval` flag ranks each accepted action with the
seat-view heuristic evaluator and prints aggregate move-quality and average
loss counters for calibration runs.

Arena can also append private AI decision traces with `-ai-trace-log`, which is
useful for long-running AI-vs-AI sessions that will be analyzed after the run.

External raw AI processes receive a JSON object containing visible state,
private hand, legal command hints, and previous parse errors. They should print
exactly one command such as `1`, `attack 6C`, `defend 1 7C`, `throw 6D`,
`pass`, `take`, `done`, or `concede`.

The Makefile keeps Go build caches under `.cache/` so commands work in
restricted workspaces without writing to the user-level Go cache.

## Planning Docs

- [PRD](docs/2026-06-10-durak-prd.md)
- [Match Configuration Draft](docs/2026-06-15-match-config-specs.md)
- [Storage Foundation](docs/2026-06-16-storage-foundation-specs.md)
- [Heuristic Position Evaluation](docs/2026-06-16-heuristic-position-evaluation-specs.md)
- [Heuristic Position Evaluation Tasks](docs/2026-06-16-heuristic-position-evaluation-tasks.md)
- [Stack](docs/STACK.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Project Rules](docs/PROJECT_RULES.md)
