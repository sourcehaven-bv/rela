---
id: IMPL-AC1I
type: implementation-checklist
title: 'Implementation: Decouple dataentry from internal/workspace type imports'
status: done
---

## Implementation

- [x] New `dataentry.WatchOptions` type using `storage.ChangeEvent` directly
- [x] `App.startWatching` field signature changes from `func(workspace.WatchOptions) error` to `func(WatchOptions) error`
- [x] `cmd/rela-server/main.go` wraps `ws.StartWatching` with 5-line adapter closure
- [x] `cmd/rela-desktop/main.go` same adapter pattern
- [x] `internal/dataentry/test_helpers_test.go` adapter inline in `rebindApp`
- [x] `internal/dataentry/app.go` drops `internal/workspace` import
- [x] `internal/dataentry/watcher.go` drops `internal/workspace` import; uses `storage.ChangeEvent` directly
- [x] `.go-arch-lint.yml` removes `dataentry → workspace`
- [x] `go test -race ./...` clean
- [x] `just lint` clean
- [x] `just arch-lint` OK
- [x] `just ci` full pipeline green

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | minor | **Addressed** | Doc comment on `WatchOptions` no longer names `workspace.WatchOptions` — talks about "the wiring site" abstractly. dataentry's docs shouldn't re-introduce the coupling the ticket removes. |
| 2 | leverage | Won't fix | Reviewer suggested checking `onDataReload` for deadness — verified it IS called (StartWatching → onDataReload). Not dead. |
| - | - | Confirmed | Adapter duplication in 3 sites: intentional. A helper would have to live in workspace (re-couples) or dataentry (defeats ticket) or a new package (over-engineering for 15 lines of wiring boilerplate). |
| - | - | Confirmed | `storage.ChangeEvent` on dataentry surface: not new — dataentry already imports `storage` for FS/RootedFS. |
