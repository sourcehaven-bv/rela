---
id: IMPL-RUUZ6S
type: implementation-checklist
title: 'Implementation: fsstore self-echo suppression is dead in production: SafeFS.OnPostWrite(RecordWrite) wiring was dropped with the encryption removal (#508)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `TestNewInstallsSelfEchoRecorder`, `TestStartWatchingRefusesUnobservableFS` (fsstore internal).
- [x] Integration tests written (test full flow, not just units) — `TestFSFactoryWatcherSuppressesSelfEcho` (internal/app): real directory, `SafeFS(OsFS)`, real fsnotify watcher, one store write + one external write, asserts the exact event and observer sequence.
- [x] Happy path implemented — `fsstore.New` installs `echoes.Recorded` on any FS satisfying the consumer-side `postWriteObservable`; `RecordWrite` deleted (FSStore 92→91 / 36→35).
- [x] Edge cases from planning handled — FS without the capability: open succeeds (CLI never watches), `StartWatching` returns `ErrWatchNeedsObservableFS`.
- [x] Error handling in place (errors surfaced, not swallowed) — the degraded mode is now an error at the seam whose guarantee it breaks, never a silent no-op.

## Test Quality

- [x] Using fixture builders or factories for test data — `newTestStore`, `recordingObserver`, the existing factory fixtures.
- [x] No hardcoded values in assertions when object is in scope — expected events built from the same ids/types as the writes.
- [x] Only specifying values that matter for the test.
- [x] Interpolated values constructed from objects, not hardcoded — paths via `paths.EntitiesDir`.
- [x] Property comparisons use original object, not hardcoded strings.

## Manual Verification

- [x] Feature manually tested end-to-end — the integration test IS the production path (FSFactory → SafeFS → fsnotify); it failed on develop with `putIDs == [POL-1, POL-1, POL-2]` and a spurious `EventEntityUpdated`, and passes after the fix.
- [x] Each acceptance criterion verified with test scenario from planning — one Subscribe event and one `EntityPut` per write: pinned.
- [x] Edge cases manually verified — opaque FS refused at `StartWatching`: pinned.

**Verification Evidence:** `go test -race -count=1` green on
internal/store/fsstore, internal/app, internal/cli, internal/dataentry,
internal/appbuild, internal/mcp, internal/attachment, internal/analysis,
internal/store/storetest. `just ci` run recorded on the review checklist.

## Quality

- [x] Code follows project patterns (check similar code) — consumer-side capability interface discovered by type assertion, like `store.Formatter` / `VersionServiceProvider`; sentinel error like `store.ErrNotFound`.
- [x] Checked for DRY opportunities — `echoes.Recorded` already had the `storage.WriteObserver` signature, so `RecordWrite` was pure indirection and is gone.
- [x] No security issues introduced — no new I/O paths; the observer receives bytes the store just wrote.
- [x] No silent failures (errors logged AND returned) — `ErrWatchNeedsObservableFS` is returned; callers (`dataentry`, `mcp` wiring) already propagate `StartWatching` errors.
- [x] No debug code left behind.
