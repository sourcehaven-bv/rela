---
id: TKT-0SP1
type: ticket
title: Migrate CLI to scoped services helper (drop package globals)
kind: refactor
priority: medium
effort: m
status: done
---

## Summary

`internal/cli/root.go` holds package-level globals `ws`, `projectCtx`, `meta`,
`out` populated via `workspace.Discover` in `PersistentPreRunE`. Subcommands
reach for `ws.X()` directly. After TKT-KWAX (MCP migrated) and TKT-LCTG /
TKT-Q1JT (workspace internals straightened), CLI is the last big consumer that
still binds to `*workspace.Workspace`.

This ticket builds a wiring helper for CLI — analogous to
`internal/cli/mcp_wiring.go` — that constructs focused services (Store, Meta,
Tracer, Searcher, Validator, EntityManager, Config, Paths, Templater, FS, Lua
deps) and exposes them via a `*cliServices` bundle. Subcommands consume services
from a context-attached bundle instead of reaching for package globals.

## In scope

- New `internal/cli/cli_wiring.go` builds `*cliServices` from the project root. Mirrors `mcp_wiring.go` (observer-wired bleve, ScriptRunner adapter, watcher adapter).
- `internal/cli/root.go::PersistentPreRunE` constructs the bundle once and stows it on the command context (or holds it as a package-level pointer that's narrower than today's `ws`).
- Each subcommand file (`create.go`, `delete.go`, `list.go`, `show.go`, etc.) reads its services from the bundle.
- Drop package globals `ws`, `projectCtx`, `meta`. Keep `out` (CLI output writer — different concern).
- `internal/cli` stops importing `internal/workspace` directly. Verified by `grep` + arch-lint.

## Out of scope

- **Lifting workspace facade methods** (`AnalyzeAll`, `CheckCardinality`, `FindDuplicates`, `FindGaps`, `FindOrphansWithScope`, `FindOrphanedTempFiles`, `CleanupOrphanedTempFiles`, `RunValidations`, `RunValidationsFiltered`, `RenameEntityType`, `AttachFile`, `ListAttachments`) into their own packages. These ~700 LOC of CLI-specific conveniences stay on `Workspace` for now. CLI consumes them via a `cliServices.Analyze`/`cliServices.Attach`/`cliServices.Rename` facade that holds a `*workspace.Workspace` reference. **Tracked in TKT-FACL** (follow-up).
- Changes to subcommand argument parsing, flag schema, output format.
- Migrating dataentry/scheduler.
- Deleting workspace methods that CLI no longer references.

## Depends on

- TKT-KWAX (PR #722, merged). Establishes the wiring-helper pattern that this ticket reuses.
- TKT-LCTG / TKT-Q1JT (merged). Provides `bleveindex.NewMem` + `app.FSFactory.AddObserver` for the wiring helper.

## Why

The CLI-bound globals are the single biggest "god-object via the side door"
pattern in the codebase per the original TKT-0SP1 scoping. Every subcommand
silently depends on whatever methods `*workspace.Workspace` exposes; renaming a
workspace method breaks 18 CLI files. After this ticket, each subcommand
declares the minimum services it needs via the bundle.

Splitting the facade-lift into a separate PR (TKT-FACL) keeps this diff
reviewable while still progressing the workspace decomposition arc.

## Risks

- **Subcommand count.** ~30 CLI files. Per-file diff is mechanical (s/ws.X()/svc.X()/) but the volume is real. Mitigation: scripted rename + audit pass; reviewer-friendly diff stat.
- **Test fixtures.** `cli/test_helpers_test.go` builds workspace; needs to migrate to a stub `cliServices`. Same pattern as TKT-KWAX's `newTestServices`.
- **Facade methods.** CLI still calls `svc.Analyze.FindOrphansWithScope(...)` etc. — those forward to `*workspace.Workspace` internally. Document the transitional shape so the next ticket (TKT-FACL) has a clear migration target.
- **`out` package global.** Output formatting is CLI-specific (not workspace-related); keep as-is for this ticket.
