---
id: IMPL-NPYI
type: implementation-checklist
title: 'Implementation: dataentry owns its data-entry.yaml watcher (remove WatchOptions indirection)'
status: done
---

## Implementation

- [x] `dataentry.WatchOptions` deleted.
- [x] `dataentry.App.startWatching` field + `NewApp` constructor parameter removed.
- [x] `dataentry.App.StartWatching` now feature-detects `a.store.(storeWatcher)` and asks the store directly. Errors logged via slog.Warn; non-watching stores get a slog.Debug for visibility.
- [x] New `storeWatcher` consumer-side interface in `internal/dataentry/watcher.go` (2 methods, but only StartWatching is needed here).
- [x] Adapter closures in `cmd/rela-server/main.go`, `cmd/rela-desktop/main.go`, `test_helpers_test.go::rebindApp` deleted.
- [x] **Workspace's `StartWatching` / `StopWatching` / `PauseWatching` / `ResumeWatching` / `WatchOptions` + `watcher` field deleted.** Last consumer is gone; per CLAUDE.md "migrate out, don't extend." Bonus 60-line deletion.
- [x] `onDataReload` removed (was no-op; only test-helper called it).
- [x] `e2e_test.go` fixed (was still passing `ws.StartWatching` to NewApp).
- [x] `go test -race ./...` clean. `just ci` green. `go test -tags=e2e` builds clean.

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | critical | **Addressed** | `e2e_test.go` still passed `ws.StartWatching` to NewApp — caught by `go test -tags=e2e`. Fixed. |
| 2 | significant | **Addressed** | `onDataReload` was no-op + dead-code-with-test-hook; deleted along with the simulateReload data-event branch. simulateReload now models only the config-path event (the only watcher branch that survives). |
| 3 | significant | **Addressed** | Workspace's `StartWatching` / `WatchOptions` / `Pause` / `Resume` / `watcher` field deleted — all four were unreachable from production after this PR. CLAUDE.md "migrate out" rule applied. |
| 4 | minor | **Addressed** | Added slog.Debug when `a.store` doesn't implement `storeWatcher` so misconfigured deployments aren't silent. |
| 5 | minor | **Addressed** | `App.StopWatching` doc clarified: it releases only the config subscription; store-watcher lifecycle is store-owned. |
| 6 | minor | **Addressed** | `App.StartWatching` doc clarified: returned error covers only config-subscriber failure; store-watcher errors logged not returned. |
| - | leverage | Acknowledged | Feature-detection pattern (storeWatcher in dataentry, storeStartStopper in cli/mcp_wiring) intentionally duplicated per CLAUDE.md "interfaces at the call site." Not factored. |
