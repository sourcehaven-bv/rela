---
id: RR-YX8B5
type: review-response
title: Recursive memoize on same key deadlocks under sync.Mutex
finding: 'The sketch in the plan holds `c.mu.Lock()` across `get`, and `memoize` calls `get` then executes `fn()` before calling `set` — if the mutex is held across `fn()` (or if `fn` itself calls `rela.cache.*`), you either self-deadlock on `sync.Mutex` or silently race. The plan needs an explicit statement: mutex is released across `fn()`, duplicate compute on race is acceptable, and recursive `memoize` with the same key is documented.'
severity: critical
resolution: 'Addressed in AC 13 of the revised plan: `Cache` uses `sync.RWMutex`; `Memoize` explicitly releases the lock across `fn` execution. Concurrent misses on the same key both run `fn`, last write wins. This is documented as a test (AC 17: ''concurrent memoize on same key: both fn calls observed, last write wins, no deadlock'').'
status: addressed
---
