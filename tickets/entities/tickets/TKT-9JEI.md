---
id: TKT-9JEI
type: ticket
title: Migrate dataentry server to wire its own services (off Workspace)
kind: refactor
priority: medium
effort: m
status: ready
---

## Summary

`internal/dataentry/App` already takes its services as constructor arguments
rather than reaching through Workspace (good — see CLAUDE.md). The remaining
coupling is at the *wiring site*: `cmd/rela-server` and `cmd/rela-desktop` build
a Workspace and read services off it to pass into `dataentry.NewApp`. Lift the
wiring so each binary constructs Store / Meta / EntityManager / Searcher /
Tracer / Validator / Templater directly and feeds them to `dataentry.NewApp`
without a Workspace in between.

## In scope

- `cmd/rela-server/main.go` constructs each focused service explicitly, passes them to `dataentry.NewApp`. No `workspace.Discover` call.
- `cmd/rela-desktop/main.go` does the same; per-project lifecycle management remains intact.
- Watcher wiring (`storage.Watcher` + index reindex hook) is set up at the wiring site rather than via `ws.StartWatching`.
- `indexer.Indexer` (or equivalent — see "extract Indexer" ticket if separate) is constructed and started at the wiring site.
- Existing dataentry tests still pass.

## Out of scope

- Changes to `dataentry.App` itself (it already accepts focused services).
- Refactor of dataentry's internal handler structure.
- The MCP and scheduler migrations (separate tickets).

## Depends on

- `entitymanager.Manager` real implementation (separate ticket).
- automation.Runner extraction (separate ticket).
- "Extract Indexer service" if that becomes a separate ticket — currently the search-reindex goroutine lives in Workspace.

## Why

Once this lands together with MCP and scheduler migrations, no production entry
point goes through Workspace. The package becomes deletable.

## Risks

- The watcher → reindex callback chain is currently hidden inside Workspace; reproducing it at the wiring site needs care to avoid losing event coverage.
- per-project lifecycle in rela-desktop (cancellable contexts per loaded project) needs to compose with focused-service wiring; verify the lifecycle doesn't leak.
- This ticket is the largest of the migration series; consider splitting if implementation reveals natural seams (e.g., one PR for the wiring extraction, another for indexer extraction).
