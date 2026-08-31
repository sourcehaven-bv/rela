---
id: RR-2LIP5S
type: review-response
title: 'MaxConns >= 2 guard comment overclaimed: each held lock pins a connection, so N concurrent holders need N+1 connections'
finding: |-
    The MaxConns < 2 refusal in AcquireKeyedLock was documented as if >= 2 were sufficient: "the lock pins one connection while the caller's writes use others". That reasoning covers exactly ONE holder.

    Because every HELD lock pins a pool connection for its whole lifetime, C concurrent holders that also need to write require C+1 <= MaxConns to make progress. At C == MaxConns every connection in the pool is a parked lock session and the holders' own writes block waiting for a connection that can only be freed by a release that is itself waiting on the write. The guard cannot detect this — it is a per-call check of a global property.

    This is a real ceiling on lock concurrency and not something a bigger constant removes; it is inherent to pinning a session per held lock. It matters because the intended consumer (TKT-1EM4KL webhook routes) is precisely a fan-out workload where many distinct keys are held at once — the case the seam exists to serve.

    Severity is minor rather than significant because the failure mode is benign: pgxpool.Acquire BLOCKS rather than erroring, so contention shows up as latency bounded by the caller's ctx deadline, never as corruption or a lost lock. Stress-verified against a live PostgreSQL: SerializesConcurrent (8 goroutines, MaxConns=2) passes consistently at -count=5, and the blocking cases at -count=3, because waiters simply queue on the pool and the lock still serializes correctly.
severity: minor
resolution: |-
    Documented at the guard in keyedlock.go (commit cd568e6c) rather than papered over, since no code change can lift the ceiling. The comment now states that the guard covers one holder and not N, gives the C+1 <= MaxConns rule, notes the symptom is ctx-bounded latency rather than corruption, and advises sizing the pool above the expected number of simultaneously-held locks — or preferring a conditional write (unique: / If-Match) when the critical section is a single write anyway.

    Deliberately NOT enforced in code: the guard is per-call and cannot see how many locks are held process-wide, so any numeric check would be theatre. The honest remedy is pool sizing at the wiring site, which is where the consumer ticket will make the decision.
status: addressed
---
