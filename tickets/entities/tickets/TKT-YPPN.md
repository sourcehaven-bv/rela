---
id: TKT-YPPN
type: ticket
title: Adopt log/slog in place of stdlib log
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

Replace stdlib `log` usage in internal library packages with `log/slog`. The
stdlib `log` package is global mutable state (via `log.SetOutput`), has no
levels, and is not safe to capture from parallel tests — adding `t.Parallel`
to any test that captures logs causes nondeterministic failures.

Enforced by a `depguard` lint rule in `.golangci.yml` that forbids `import
"log"` package-wide. `log/slog` is explicitly allowed. `internal/mcp/server.go`
is exempted because it must bridge to `mcp-go`'s `WithErrorLogger` which takes
a `*log.Logger`; it constructs one via `slog.NewLogLogger` so everything still
flows through the slog handler.

Entry points (`cmd/rela/main.go` indirectly via `internal/cli/root.go`,
`cmd/rela-server/main.go`, `cmd/rela-desktop/main.go`) wire `--verbose` /
`--quiet` flags to `slog.LevelDebug` / `slog.LevelWarn` at startup.
