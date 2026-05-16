---
id: REV-6W33
type: review-checklist
title: 'Review: dataentry owns its data-entry.yaml watcher (remove WatchOptions indirection)'
status: done
---

## Code Review

- [x] cranky-code-reviewer run on the diff
- [x] 1 critical finding addressed (e2e_test.go was broken; `go test -tags=e2e` catches it now)
- [x] 2 significant findings addressed (onDataReload removed; workspace.StartWatching et al. deleted)
- [x] 3 minor findings addressed (slog.Debug on missing watcher; doc clarifications on Start/StopWatching)
- [x] Tests pass under `-race`
- [x] `just test -tags=e2e` builds clean
- [x] `just ci` green

## Disposition

See IMPL-NPYI for the full table.

**Headline outcomes:**

- **Net 111-line deletion** across 8 files. The watcher indirection that TKT-B8ZJ partially decoupled is now gone entirely.
- **Workspace shed 60+ lines.** `StartWatching` / `StopWatching` / `PauseWatching` / `ResumeWatching` / `WatchOptions` type / `watcher` field all deleted. They were unreachable from production after this PR; per CLAUDE.md "migrate out, don't extend" the right move is delete-now, not wait-for-TKT-64R3.
- **Build-tag catch.** Cranky ran `go test -tags=e2e` and found a file I'd missed — `e2e_test.go` still passed the now-removed parameter. Fixed.
- **`onDataReload` removed.** Was a no-op called only by test infrastructure modeling a code path that no longer exists in production.
