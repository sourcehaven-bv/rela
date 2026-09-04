---
id: RR-C1ALTT
type: review-response
title: reload() after Close() resurrects services on a closed store
finding: storage.Watcher.Stop does not await an in-flight OnChange, so a schema event mid-flight at shutdown could make reload() assemble a new service generation (job queue, mail worker, GC sweep) against a store Close had already torn down, leaking them permanently.
severity: critical
resolution: Added a `closed` flag set under mu at the top of Close; reload() refuses when set and re-checks after assembling, retiring the successor via CloseAssembly if superseded.
status: addressed
---

**Finding:** `storage.Watcher.Stop` only closes `done` and the fsnotify handle —
it does not wait for an in-flight `OnChange`, which runs synchronously after a
200ms debounce. So a schema event can be mid-flight when `Close` begins: the
callback blocks on `s.mu`, `Close` completes, then `reload()` proceeds and
assembles a new generation (job queue, mail worker, GC sweep) against a
torn-down store. Nothing would ever close them; on postgres that is a leaked
connection pool and LISTEN connection per occurrence.

**Resolution:** Added a `closed` flag on `mcpServices`, set under `mu` at the
top of `Close` before any teardown. `reload()` returns an error when it observes
it, and re-checks after building the successor (retiring it with `CloseAssembly`
if shutdown or another reload landed meanwhile). `watchSchema` already
logs-and-continues on error, so this degrades correctly.

Pinned by `TestMCPServices_ReloadAfterCloseIsRefused` and
`TestMCPServices_WatchSchemaAfterCloseDoesNotResurrect`; both were confirmed to
fail without the guard.
