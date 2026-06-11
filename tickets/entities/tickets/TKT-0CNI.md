---
id: TKT-0CNI
type: ticket
title: Migrate CLI from cobra to kong
kind: refactor
priority: medium
effort: l
status: done
---

Migrates all ~50 `rela` CLI subcommands from `github.com/spf13/cobra` to
`github.com/alecthomas/kong`.

Kong's struct-tag-driven approach removes cobra's RunE signature constraint and
the package-global flag/context machinery that came with it:

- **Removed**: `cliReadFromContext` / `cliWriteFromContext` / `cliAnalyzeFromContext` (ctx-attached service-locator); the `cliRead` / `cliWrite` / `cliAnalyze` interfaces; ~30 package-global flag vars; `applySeeder` / `testCtx` / `testCmd` test fixture machinery; cobra `PersistentPreRunE` project-discovery wiring.
- **Added**: per-subcommand `XxxCmd struct { ... }` with `arg:""` / `name:"..."` field tags. Each command's `Run(ctx, svc *cliServices) error` method receives services kong-bound at the wiring site.
- **Net diff**: -2,233 LOC across 49 files.

All Long/Examples cobra help blocks dropped — one-line help only via kong tags.

CI green: build, lint (0 issues), arch-lint, tests, coverage at 74.7% (up from
71.1%).

Known follow-ups:
- `rela completion` is currently stubbed pending a `github.com/willabides/kongplete` integration.
- `rela graph -o` short-flag dropped (collides with root `--output`); long-flag renamed `--file`.
