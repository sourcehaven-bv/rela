---
id: IMPL-XGSYWM
type: implementation-checklist
title: 'Implementation: Keyed lock seam (internal/lock)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

New package `internal/lock`: `lock.go` (the `Locker` seam + `ValidateKey`),
`memlock.go` (in-process backend), `pglock.go` (`BackendLocker` adapting a store
capability), `locktest/locktest.go` (shared conformance suite). New
`internal/store/pgstore/keyedlock.go` (`AcquireKeyedLock`, the `KeyedLocker`
capability + `KeyedLockerFor` discovery).

The "integration test" for the postgres half is the DB-gated suite in
`keyedlock_test.go`, which runs the SAME `locktest.RunAll` against a live
database plus four cross-store cases (exclusion across stores, distinct keys not
contending across stores, per-schema scoping, capability discovery).

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Property comparisons use original object, not hardcoded strings

Both backends are driven through one `locktest.Factory`, so the contract is
asserted once and shared rather than restated per backend.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] ~~Each acceptance criterion verified with test scenario from planning~~
(N/A: no planning checklist — ticket was created and implemented in one session
at the user's request.)
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test -race -count=3 ./internal/lock/...` — pass. The race detector is the
point for a lock; 3 runs to shake out ordering flakes.
- `internal/lock` at **100% statement coverage** (`go tool cover -func`).
- All four build tags compile: default, `-tags postgres`, `-tags sqlite`,
`-tags memorybackend`.
- `just arch-lint` — OK, no warnings. `internal/lock` registered as a leaf
component with no `mayDependOn` entry (it depends on nothing).
- `just lint` — exit 0. `just comment-lint` — clean across 11485 comments.
- `just plimsoll` — clean after bumping `pgstore.Store` 49->50 / 39->40 with a
rationale note.

**Postgres backend verified against a live server** (PostgreSQL 15, local
Postgres.app). An earlier revision of this checklist claimed the postgres path
could not be verified locally, on the strength of `RELA_TEST_DATABASE_URL` being
unset — a server was in fact running. That mistake mattered: the skip was hiding
a real defect (RR-U99GDV, `pg_advisory_lock` cast to `::bigint`, which does not
resolve) that no local gate could catch.

With `RELA_TEST_DATABASE_URL` set, `go test -race -count=1
./internal/store/pgstore/` passes in full (94s), including:

- `TestKeyedLock_Conformance` — every `locktest` case against the real backend.
- `TestKeyedLock_ExclusiveAcrossStores` — two stores (standing in for two
  processes) exclude each other on one key.
- `TestKeyedLock_DistinctKeysDoNotContendAcrossStores` — the burst property.
- `TestKeyedLock_ScopedPerSchema` — tenant isolation; the same key in two
  schemas is two locks.
- `TestKeyedLock_CancelledAcquireDoesNotWedgeOrLeak` — added in review; drives
  10 cancelled acquires (more than `MaxConns`) while a holder keeps the key,
  then asserts the key is free and the pool still serves. A connection leak
  exhausts the pool rather than passing by luck.

The test gating was also corrected: `skipOrFailWithoutDSN(t)` was being called
unconditionally, so the SQL never reached a server under either mode. It now
uses `testDSN(t)`, which checks the env var before deciding to skip.

Three edge cases handled deliberately:

1. **Cancelled acquire may still have been granted.** PostgreSQL can grant the
lock exactly as cancellation arrives, so the session may hold a lock the call is
about to report as failed. The connection is destroyed (`Hijack().Close()`)
rather than pooled — a session-scoped lock dies with its session, so the key
releases instead of being held by a pooled connection nobody knows is a holder.
Returning it to the pool would wedge the key.
2. **Memory-backend map growth.** Keys derive from request data, so a naive map
grows one entry per distinct key ever locked and never shrinks — invisible in a
test using three keys, unbounded in production. Entries are refcounted;
`TestMemoryLocker_NoEntryLeak` and `TestMemoryLocker_ConcurrentDistinctKeys`
assert the map returns to zero.
3. **Abandoned waiter.** A caller that gives up waiting must not leave the mutex
locked with no owner once the holder releases.
`TestMemoryLocker_AbandonedAcquireReleases` pins it.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the `state.KV` / `jobs.Queue` precedent: narrow interface, per-tier
backends chosen at the wiring site, shared conformance kit excluded from
coverage. The postgres backend follows `TryMigrationLock` closely — same
`MaxConns < 2` refusal, same one-pinned-connection rule, same
`context.WithoutCancel` release.

One deliberate departure, documented at the call site: **blocking rather than
try-or-skip.** Every existing pgstore advisory-lock caller uses
`pg_try_advisory_lock` and skips when another holder is active — right for a
sweep (someone else is doing the same work), wrong for a request path where
skipping silently drops the caller's work.

`NewBackendLocker` rejects a nil backend, per the constructors-reject-nil rule:
a nil backend would yield a Locker that serializes nothing, the one failure mode
a lock must never have.

**Not wired.** Nothing consumes the seam yet — that belongs with TKT-1EM4KL,
which needs it. Wiring a locker nothing calls would be speculative.
