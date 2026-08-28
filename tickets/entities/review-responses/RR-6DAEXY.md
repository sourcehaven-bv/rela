---
id: RR-6DAEXY
type: review-response
title: Warn-once cache never re-warned after a mode regression, and grew unbounded
finding: |-
    Two defects in the warn-once cache.

    (1) Keyed on path alone, so once a file warned at 0640 the entry was permanent. An operator who chmods to 0600 and later regresses to 0666 — world-WRITABLE, strictly worse than first reported — got no second warning for the process lifetime, and the log still named 0640. "Don't spam" and "warn once ever" are not the same requirement.

    (2) The sync.Map never evicts. In the SharedBase multi-tenant case it retains one entry per project path forever, keyed by a value whose cardinality the package does not control. Small, but it is the shape of leak that matters later — and keying on (path, mode) to fix (1) makes it marginally worse.
severity: significant
status: addressed
---

## Resolution

**(1)** Introduced a `warnKey{path, perm}` struct as the cache key. A changed
mode warns again; a mode that simply stays wrong stays quiet — which is the
requirement the original comment was reaching for.

`TestLoad_WarnsAgainWhenModeChanges` walks 0640 → 0600 → 0666 and asserts two
warnings, with the second naming `0666`. That test fails against the old
path-only key.

**(2)** Bounded the map at `maxWarnedPaths = 1024`, tracked with an
`atomic.Int64`. Past the cap the entry is rolled back and the warning dropped —
acceptable for an advisory diagnostic, and preferable to unbounded retention.
The counter increments after `LoadOrStore` so each distinct entry counts once;
racing goroutines may overshoot slightly, which the comment states, because the
bound exists to stop growth rather than to be exact.

Concurrency was separately confirmed correct rather than assumed:
`TestLoad_ConcurrentLoadsWarnOnce` fires 16 concurrent `Load` calls at one
permissive file and asserts exactly one warning, under `-race`.
