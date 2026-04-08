# Stress / soak / fuzz runner for rela-server

A browser-driven workload generator with two modes:

- **stress**: N concurrent browser sessions driven through a scenario for
  a fixed duration. Good for soak tests, reproducing timing-sensitive
  bugs, and measuring tail latency under load.
- **fuzz**: a single browser session driven by a fast-check
  property-based fuzzer. Generates random sequences of UI actions and
  shrinks any failing sequence to a minimal counter-example. Good for
  finding rare state-transition bugs.

Both modes spin up a fresh `rela-server` against an isolated `/tmp` copy
of a real project. The schema canary (stress mode) or per-sequence
oracles (fuzz mode) measure server-side latency independently of the
browser. When an invariant breaches, the runner captures a goroutine
dump from the server's pprof endpoint and writes it next to the report.

## Why a real browser

The hangs we're chasing involve SSE (`/api/v1/_events`), the security
middleware, the live-reload lock, and Firefox's parallel-fetch behaviour
all interacting at once. A pure-Go test cannot reproduce that. The runner
launches Chromium via Playwright, so every request goes through the real
browser stack including `EventSource`.

## Running

```bash
cd frontend
npm install                          # ensure playwright + fast-check deps

# Stress mode
npm run stress -- --scenario=watcher-pressure --duration=30s --users=4

# Fuzz mode
npm run stress -- --mode=fuzz --browser=firefox --num-runs=200 --max-actions=15
```

Common flags (both modes):

| Flag | Default | Meaning |
|---|---|---|
| `--mode=stress\|fuzz` | `stress` | Which mode to run |
| `--browser=chromium\|firefox\|webkit` | `chromium` | Browser engine |
| `--source-project=PATH` | `tickets` | Project to copy into `/tmp` for the run |
| `--seed=N` | random | RNG seed for reproducibility |
| `--report-dir=PATH` | `/tmp/rela-{mode}-{timestamp}` | Where to write logs and dumps |

Stress-mode flags:

| Flag | Default | Meaning |
|---|---|---|
| `--scenario=NAME` | `watcher-pressure` | Which scenario to run (see `scenarios/`) |
| `--duration=DUR` | scenario default | How long to run the workload (`10s`, `2m`, ...) |
| `--users=N` | scenario default | Number of concurrent browser contexts |

Fuzz-mode flags:

| Flag | Default | Meaning |
|---|---|---|
| `--num-runs=N` | `200` | fast-check sequences to try before giving up |
| `--max-actions=N` | `15` | Max actions per sequence |
| `--oracle-timeout=DUR` | `10s` | Max time for the post-sequence liveness nav |
| `--action-budget=DUR` | `3s` | Max time for any single atomic action (goto/click/reload/back). Exceeding this fails the sequence — this is the "long long time hang" guard. |
| `--mutate` | `false` | When set, `create-entity` and `edit-entity` actually submit the form and write to the /tmp project copy. Off by default because mutations expose state-dependent bugs that build up across iterations. Enable when investigating save/edit/automation races specifically. |

## Scenarios (stress mode)

| Scenario | Purpose |
|---|---|
| `watcher-pressure` | One foreground browser doing reads while a background goroutine touches files on disk to fire the watcher. **Designed to reproduce BUG-FMS1.** |

Add a new scenario by dropping a file in `scenarios/` that exports a
`Scenario` object — see the existing file for the shape.

## Fuzz action vocabulary

The fuzzer picks randomly from these actions (see `fuzzRunner.ts`):

| Action | Description |
|---|---|
| `goto(list)` | Navigate to `/v2/list/<list>` |
| `click-row(index)` | Click the Nth entity row on the current list |
| `reload` | Reload the current page |
| `back` | `history.back()` |
| `wait(ms)` | Sleep N milliseconds — not measured against the latency budget |
| `touch-file` | Touch a random entity file on disk to fire the server's file watcher |
| `create-entity(type)` | Open the create form, fill the title; submit iff `--mutate` is set |
| `edit-entity(rowIndex)` | Click a row, click Edit, replace the title; submit iff `--mutate` is set |

**Mutation mode**: When `--mutate` is set, `create-entity` and
`edit-entity` actually write to the project via the submit button. The
fuzzer always runs against a per-run `/tmp` copy of the source project
(see `serverProcess.ts`), so the writes are safe — the copy is cleaned
up when the run ends. **Never set `--source-project` to a directory
you care about when `--mutate` is on.**

Mutation mode exercises interactions the read-only mode can't reach:
the file watcher, SSE refresh cascades, metamodel automations
(auto-created checklists, property defaults), and slug-based file
renames. It also exposes state-dependent bugs that build up across
iterations, so it's more likely to find deeper issues — and more
likely to need follow-up investigation.

## Invariants

### Stress mode

Thresholds are per-scenario. A run fails (exits non-zero, dumps
goroutines) if any is breached.

- No HTTP 5xx, ever.
- p99 latency for `/api/v1/_schema` < threshold (default 200 ms).
- p99 latency for entity create/update < 1 s.
- No browser console errors.
- Goroutine count growth bounded.

### Fuzz mode

Each sequence is evaluated against three oracles; any failure flags the
sequence and fast-check shrinks it to a minimal counter-example.

1. **Per-action latency budget** (default 3 s). Every atomic navigation
   action (goto/click-row/reload/back) that exceeds the budget fails
   the sequence. This is the "long long time hang" guard.
2. **Post-sequence liveness nav** (default 10 s). After the sequence
   replays, a fresh `page.goto(/v2/list/all_tickets)` must complete and
   show at least one entity row within the timeout. Catches "sequence
   left the SPA in a permanently broken state" cases.
3. **No non-benign console errors** during the sequence. The benign
   filter (see `BENIGN_PATTERNS` in `fuzzRunner.ts`) silences known
   transient browser-internal warnings (EventSource reconnect, Vite
   chunk-load races that auto-recover via reload, Firefox in-flight
   request cancellations on navigation). Anything else fails.

On failure, the runner captures pprof goroutine/heap/mutex/block
profiles and writes them to the report dir.

## Future extensions

- A `save-entity` fuzz action distinct from `create-entity` / `edit-entity`
  that explicitly DOES submit the form. Requires careful cleanup between
  iterations or a model-based approach to track expected row counts.
- Multi-tab fuzzing: open a second BrowserContext and switch between
  tabs mid-sequence. Catches SSE cross-tab interaction bugs.
- A CI variant: a 30-second deterministic distillation runnable from
  `npm run test:e2e`, with the same action vocabulary but a fixed seed
  and tighter timeout.
- Feed failed counter-examples into a regression corpus so CI replays
  historical bugs at every run.
