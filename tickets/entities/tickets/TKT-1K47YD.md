---
id: TKT-1K47YD
type: ticket
title: 'Keyed lock seam (internal/lock): named mutual exclusion with per-tier backends'
kind: enhancement
priority: medium
effort: m
status: review
---

A narrow seam for **named mutual exclusion**: callers ask for a lock on a string
key, do work, release. Backends are chosen at the wiring site per deployment
tier, exactly as `store.Store`, `state.KV`, `jobs.Queue` and `userstate.Store`
already are.

Extracted from [[TKT-1EM4KL]] (declarative webhook routes), which needs it, but
it is not webhook-specific — it is a general capability rela lacks.

## Why

rela's only mutual exclusion today is `writeMu`: **one process-wide mutex**
covering the whole data-entry write surface, held for the full duration of a Lua
action. That has two consequences:

- **Too coarse.** Unrelated writes serialize against each other. Under a
concurrent burst (an Icinga outage fans out hundreds of parallel POSTs — its
notifications run on an uncapped `boost::asio` thread pool) requests queue and
time out. TKT-X06LA2 was literally "fix the writeMu DoS" on this surface.
- **Too narrow in scope.** It is per-PROCESS. `docs/postgres-backend.md`
documents several rela-server processes against one database, where `writeMu`
provides nothing at all.

A keyed lock fixes both: contention only between operations that name the SAME
key, and — on postgres — across processes.

## Interface

Small, in the `state.KV` / `jobs.Queue` mould:

```go
// Locker provides named mutual exclusion.
type Locker interface {
    // Acquire blocks until the lock named key is held, ctx is done, or the
    // backend fails. The returned release function is idempotent.
    Acquire(ctx context.Context, key string) (release func(), err error)
}
```

Design points to settle in review:

- **Blocking, not try-or-skip.** Every existing advisory-lock caller in pgstore
uses `pg_try_advisory_lock` and skips on failure — correct for a sweep (another
process is already doing it), wrong for a request path, where skipping means
silently dropping work. Bounded waiting is the caller's job via ctx deadline.
- **Release must be idempotent** and safe to `defer`.
- **Key validation is the seam's job**, mirroring `state.ValidateKey`: keys
become filesystem paths on one backend and hash inputs on another, so the
contract holds every backend to the stricter rule rather than letting a key work
on one tier and fail on the next.
- Whether a lock is **reentrant** — propose NOT (a session-scoped postgres
advisory lock is reentrant per session, an fs lock is not; promising the weaker
contract keeps backends honest).

## Backends

| Tier | Implementation | Scope |
| --- | --- | --- |
| memory (default/desktop) | `sync.Mutex` map, refcounted | in-process |
| postgres | `pg_advisory_lock(classID, hashtext(schema \|\| key))` | cross-process |

Redis or an fs/flock backend are possible later; the seam exists so they need no
consumer changes. **Do not build them speculatively** — memory + postgres map
onto the two tiers that exist.

The postgres backend follows the established idiom: rela already has four
advisory-lock keys (`migrate`, `reconcile`, `migration`, `sweep`) using
`pg_try_advisory_lock($1::int, hashtext(current_schema()))` — a two-int lock
whose second slot is already schema-scoped for tenant isolation. A keyed lock is
that same shape with a payload-derived key in the second slot.

Two constraints from the existing callers:

1. **Advisory locks are SESSION-scoped**, so acquire/release must run on ONE
acquired connection held for the lock's lifetime — not issued via the pool.
`sweep.go` warns that going through the pool "silently voids the single-writer
guarantee." This means the postgres backend holds a pooled connection while the
lock is held, so a lock leak is a connection leak: the release path must be
robust, and a bounded acquire timeout is not optional.
2. **Use a distinct class id** from the existing four keys, since advisory locks
are database-global.

## Conformance suite

`internal/lock/locktest` with `RunAll`, matching `statetest` / `jobstest`. Any
new backend must pass it. At minimum: mutual exclusion on the same key; NO
contention between different keys (the property the whole design rests on); ctx
cancellation while waiting; idempotent release; release-on-panic; key validation
parity.

## Out of scope

- Distributed consensus, fencing tokens, lease renewal. This is mutual
exclusion, not a lock service with liveness guarantees. A postgres advisory lock
dies with its session, which is the correct failure mode here.
- Deadlock detection. Callers take one lock at a time; if that stops being true,
revisit.
- Rewiring `writeMu` onto this seam. Tempting, but `writeMu`'s replacement is
already claimed by the DEC-8UIL0 `Tx` arc — keep the two separate.
