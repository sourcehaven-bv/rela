---
id: BUGA-QFW7S6
type: bug-analysis-checklist
title: 'Analysis: fsstore self-echo suppression is dead in production: SafeFS.OnPostWrite(RecordWrite) wiring was dropped with the encryption removal (#508)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — `TestFSFactoryWatcherSuppressesSelfEcho` (internal/app/factory_test.go) fails on develop: observer sees POL-1 twice and a spurious `EventEntityUpdated` for POL-1 precedes the external create.
- [x] Minimal reproduction steps documented — open via `app.FSFactory` over `SafeFS(OsFS)`, `StartWatching`, `CreateEntity`, observe events.
- [x] Environment/conditions noted — any process that starts the fsstore watcher (rela-server data-entry, MCP); CLI paths are unaffected because they never watch.

## Root Cause

- [x] Immediate cause identified (why1) — no production caller of `OnPostWrite(RecordWrite)`.
- [x] Contributing factors found (why2-3) — PR #508 deleted the wiring with the encryption block it was framed as part of; the only test performed the wiring itself.
- [x] Systemic cause explored (why4-5) — the wiring lived in a different package from the type whose correctness depended on it, and the review that noticed (RR-IL1WV) deferred it as pre-existing with no ticket.

## Fix Planning

- [x] Fix approach determined — install `echoes.Recorded` inside `fsstore.New` via a consumer-side `postWriteObservable` capability (SafeFS and MemFS both satisfy it); delete `RecordWrite`; `StartWatching` returns `ErrWatchNeedsObservableFS` when the FS could not be observed, so the degraded mode is unreachable.
- [x] Regression test planned — production-path test in `internal/app` (real fsnotify, positive wait on an external write) plus two fsstore-internal pins (New wires echo; StartWatching refuses an opaque FS).
- [x] Related areas checked for similar issues — every production `fsstore.New` goes through `app.FSFactory`; all production FS handles are `SafeFS` (appbuild.go:1178/1199, rela-desktop main.go:856). `cli/acl_test.go` uses a bare OsFS via appbuildtest but never starts the watcher.
