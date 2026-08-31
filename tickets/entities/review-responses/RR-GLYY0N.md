---
id: RR-GLYY0N
type: review-response
title: 'Verified correct: memlock refcount/eviction, abandoned-waiter goroutine, and the Hijack().Close() paths'
finding: |-
    Recording the audit of the three subtle paths the review was specifically asked to scrutinise, so a later reader knows they were examined rather than skimmed. All three are CORRECT AS WRITTEN — noted explicitly because each looks wrong at a glance and is a plausible target for a well-meaning 'fix' that would introduce a bug.

    1. memlock.go refcount/eviction. entry() increments e.waiters under l.mu BEFORE the caller does any waiting, and drop() decrements under the same lock, deleting only at waiters==0 and only when the map still maps key to that exact entry (the `cur == e` identity check). So an entry can never be evicted while any goroutine holds or waits on its mutex, and a racing Acquire that already installed a replacement cannot have it evicted by a late dropper. Every path that calls entry() has exactly one matching drop(): the success path drops inside releaser's sync.Once, the abandoned path drops in its cleanup goroutine. drop is therefore never called more times than entry.

    2. The abandoned-waiter goroutine. When ctx fires while blocked on e.mu.Lock(), Acquire returns an error but spawns a goroutine that waits for the abandoned acquisition to complete, unlocks, and drops. This cannot leak (it is blocked on a mutex some holder will release) and cannot double-unlock (the acquiring goroutine closes `acquired` exactly once after a successful Lock, and the cleanup Unlock is the sole unlock for that acquisition — no releaser was ever handed to the caller on this path).

    Verified empirically, not just by inspection: a stress probe (64 goroutines x 400 iterations over a 4-key space, half with sub-millisecond deadlines so entries are constantly created, abandoned and evicted under contention) run under -race with -count=2. No overlap in any key's critical section, no double-unlock, and the map drained to zero entries.

    3. keyedlock.go Hijack().Close(). Double-close is unreachable: pgxpool's Conn.Hijack() panics if called twice (it nils c.res), but the two call sites are mutually exclusive — the failed-acquire site returns immediately and no releaser is ever created, and the failed-unlock site is inside releaseOnce.Do. Neither can run twice on the same conn, and the success path calls conn.Release() instead. Destroying rather than pooling is the RIGHT call: the server may grant the lock exactly as cancellation arrives, so returning that connection to the pool would wedge the key behind a holder nobody can identify.

    4. locktest's ability to catch a degraded backend. Verified by mutation testing rather than assumed: a globalMutexLocker (ignores the key, serializes everything on one mutex) is caught by DistinctKeysDoNotContend with its intended diagnostic, and a non-excluding locker that hands out every key immediately is caught independently by SameKeyExcludes, SerializesConcurrent and ContextCancelWhileWaiting. The suite does what its package doc claims.
severity: nit
resolution: |-
    No code change required for items 1, 2 and 4 — they are correct, and this entity records the verification so the reasoning is not re-litigated from scratch.

    Item 3 gained a regression test: TestKeyedLock_CancelledAcquireDoesNotWedgeOrLeak (committed 9677c964) drives 10 cancelled acquires against a live PostgreSQL while a holder keeps the key, then asserts the key is free and the pool (MaxConns=2) still serves further acquires. The loop deliberately runs more times than MaxConns so a connection leak exhausts the pool rather than passing by luck. This path has no in-process equivalent, so locktest could never have covered it.
status: addressed
---
