---
id: TKT-LK1J
type: ticket
title: Inject *slog.Logger into ai.Provider for parallel-test-safe log capture
kind: refactor
priority: low
effort: s
status: ready
---

## Goal

Replace `slog.Default()` calls inside `internal/ai` with an injectable
`*slog.Logger` field on `OpenAICompatProvider` so tests can capture log output
from a per-test logger instead of swapping the process-global default via
`slog.SetDefault`.

Resolves the deferred finding `RR-QH8C` (code review F10) from `TKT-YBKB`.

## Background

The `internal/ai` package emits structured logs via `slog.Debug`, `slog.Info`,
and `slog.Warn` from `logRequestStart`, `logRequestSuccess`, and
`logRequestFailure`. These all call into the process-global `slog.Default()`.

Tests that want to assert on log output (`TestProvider_Chat_LogsSuccess
AndFailure`, `TestProvider_Chat_LogsFailure`, the entire
`TestProvider_Chat_KeyNeverLeaks*` family) capture logs via the `captureLog`
helper, which swaps `slog.Default()` to a buffered handler and serializes
through a package-level mutex `logMu`.

This works today only because no test in the package calls `t.Parallel()`. The
moment someone adds it, the global mutex won't help — another test running in
parallel can emit a log line via `slog.Default()` while a `captureLog`-using
test holds the lock, contaminating the captured buffer with unrelated output.
The `KeyNeverLeaks` family is especially fragile: a stray log line from another
test could either accidentally satisfy or accidentally fail the leak assertion.

The cranky-code-reviewer flagged this as F10 in the TKT-YBKB code review
(`RR-QH8C`). It was deferred because the captureLog rewrite for slog (during the
develop rebase) does not actually fix the race — it only renames it. The real
fix is dependency injection.

## In Scope

- Add a `*slog.Logger` field to `openAICompatProvider` (default to
`slog.Default()` if not set so existing callers don't break)
- Add a `WithLogger(*slog.Logger) Option` mirroring the existing
options pattern, OR add a `Logger` field on `Config`/the constructor signature —
pick whichever is least invasive at the call sites
- Replace the three `logRequest*` package-level functions with methods
on `*openAICompatProvider` so they can use `p.logger` directly
- Rewrite `captureLog` in `openai_test.go` to construct a per-test
`*slog.Logger` and pass it to the provider via `WithLogger`. The `logMu` global
serialization disappears.
- Once the global mutex is gone, opt the noisier tests into
`t.Parallel()` to demonstrate the fix is real

## Out of Scope

- Changing the format of any log lines (the existing `slog.TextHandler`
output is fine)
- Wiring a custom logger from CLI/MCP entry points — they continue to
use `slog.Default()` via the constructor default. Logger DI here is about
test-isolation, not about making production loggers configurable per-call.
(That's a separate concern, see below.)
- Removing the `--verbose`/`--quiet` Cobra flag wiring from
`internal/cli/root.go` (added by FEAT-VH7Z / PR #328)

## Acceptance Criteria

1. `OpenAICompatProvider` has a `logger *slog.Logger` field that
defaults to `slog.Default()` when not set.
2. `WithLogger(*slog.Logger) Option` (or equivalent) lets callers
inject a logger.
3. The three `logRequest*` functions become methods on the provider
and use `p.logger` instead of `slog.Default()`.
4. `captureLog` no longer swaps the process-global default and no
longer uses `logMu`. Each test that wants log capture constructs its own
`*slog.Logger` over a `*bytes.Buffer`.
5. At least one log-capturing test (e.g.
`TestProvider_Chat_KeyNeverLeaks_SuccessPath`) opts into `t.Parallel()` and
still passes deterministically.
6. `go test -race ./internal/ai/...` passes.
7. No production behavior change: CLI/MCP entry points still emit
logs via `slog.Default()` as today (no entry-point wiring change).

## Notes

- This is a small mechanical refactor. The whole change should be under
~150 lines of Go.
- The `ai.LoadProvider` convenience helper does not need to change —
it builds a default-logger provider, and entry points that want a custom logger
can still use `NewOpenAICompatProvider(cfg, WithLogger(...))` directly.
- A future ticket could thread a per-call `*slog.Logger` through the CLI
for `--verbose` runs that want richer AI-specific logging, but that is not this
ticket.
