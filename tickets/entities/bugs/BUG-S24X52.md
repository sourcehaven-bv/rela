---
id: BUG-S24X52
type: bug
title: 'fsstore self-echo suppression is dead in production: SafeFS.OnPostWrite(RecordWrite) wiring was dropped with the encryption removal (#508)'
description: 'FSFactory.OpenStore never installs SafeFS.OnPostWrite(store.RecordWrite), so the fsstore watcher treats every self-write as an external edit: extra parse, re-index, duplicate observer notify and a second EventEntityUpdated on Subscribe. Regression from PR #508 (encryption removal) which deleted the wiring block.'
priority: medium
effort: s
why1: app/factory.go:66 constructs the FSStore but never calls safeFS.OnPostWrite(s.RecordWrite); the only caller of OnPostWrite with RecordWrite is watcher_internal_test.go:35, whose comment claims to 'mirror production wiring'.
why2: Commit ea99547e (#508, remove in-process at-rest encryption) deleted the SafeFS type-assert block in FSFactory.OpenStore (old lines 142-188) that installed the observer, because the block was framed as an ENCRYPTION requirement (ErrEncryptedRepoNeedsSafeFS) rather than a watcher requirement.
why3: The echo LRU's correctness was only asserted by a test that performs the wiring itself, so no test exercised the production construction path with a running watcher; a review finding (RR-IL1WV, TKT-3TA1H era) had already deferred this exact gap as pre-existing.
why4: The observer wiring lived in app.FSFactory, a different package from fsstore, whose watcher correctness depended on it; a cross-package invariant with no test on the production construction path is exactly what a refactor deletes without noticing.
why5: Review findings deferred as 'pre-existing' (RR-IL1WV) had no ticket to land in, so the known gap fell out of the tracker and re-surfaced only during an unrelated survey months later.
prevention: 'Wiring moved INTO fsstore.New (consumer-side postWriteObservable capability) so no construction site can forget it; StartWatching fails closed with ErrWatchNeedsObservableFS instead of running degraded; TestFSFactoryWatcherSuppressesSelfEcho exercises the production path with a real watcher (AM-fsstore-self-write-single-event-via-factory). Process: a ''pre-existing, deferred'' review finding must get its own bug ticket, not a deferral note.'
status: done
---

Found while surveying `fsstore.FSStore` for [[TKT-N0IKN9]]. Verified against
develop (e0187047) and the git history.

## Symptom

In `rela-server` / MCP (anywhere `StartWatching` runs —
`internal/dataentry/watcher.go:26`, `internal/cli/mcp_wiring.go:101`), every
fsstore self-write is seen by the fsnotify watcher as an EXTERNAL edit:
`reconcileEntityPath` (`internal/store/fsstore/watcher.go:179`) reads the file
back, `echoes.IsEcho` misses (nothing was ever `Recorded`), the entity is
re-parsed, re-indexed, `notifyPut` re-fires to observers (search re-index), and
a second `EventEntityUpdated` is emitted on the subscription — after the write
path already emitted `EventEntityCreated`/`Updated` under `mu`. Net: one extra
parse + index + observer pass + SSE event per write, and the create→update
double-fire is observable by SSE clients.

## Evidence

- `grep -rn "OnPostWrite(" --include='*.go' . | grep -v _test.go` — zero
callers. `RecordWrite` (`fsstore.go:229`) has no production caller.
- `git show ea99547e -- internal/app/factory.go` removes
`safe.OnPostWrite(s.RecordWrite)` (old line 188) along with
`ErrEncryptedRepoNeedsSafeFS`.
- `internal/appbuild/appbuild.go:1178,1199` construct `SafeFS` without an
observer.
- `watcher_internal_test.go:35` wires it manually ("Mirror production wiring").

## Fix (decide deliberately, don't just re-add the line)

Two honest options:

1. **Wire it.** `FSFactory.OpenStore` (`internal/app/factory.go:66`) installs
`OnPostWrite(s.RecordWrite)` when `f.FS` supports it. Make the capability a
consumer-side interface (`interface{ OnPostWrite(storage.WriteObserver) }`)
rather than a `*storage.SafeFS` type assertion — the silent-cast failure mode
was RR-Z2YI2 last time. `MemFS` supports it too, so tests exercise the same
path.
2. **Delete the observer path** and rely on the watcher's own
`echoes.Recorded` after reconcile, if the double-fire is judged acceptable. Then
`RecordWrite`, the `echoes` recording in `SafeFS`, and the test go.

Option 1 is the design the code documents (`markdown.go:439`, `echo.go:25`,
`layout.go:71`). Whichever is chosen, the fsstore decomposition tickets that
follow must not bake the dead field in.

## Preventive measure

A test that constructs the store THROUGH `FSFactory.OpenStore` (production
path), starts the watcher, performs a write, and asserts exactly ONE event is
delivered on `Subscribe` and `notifyPut` fires once. Register it as the
`adds-measure` automated-measure on this bug.
