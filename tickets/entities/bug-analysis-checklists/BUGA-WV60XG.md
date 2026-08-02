---
id: BUGA-WV60XG
type: bug-analysis-checklist
title: 'Analysis: pgstore migrate and write advisory locks are not schema-qualified'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced against a real throwaway PostgreSQL 15 by holding schema A's migrate
(and separately, write) advisory lock inside an open transaction, then driving
the real `pgstore.Migrate` / `Store.Tx` against schema B. Both blocked until the
10s test timeout instead of completing.

Minimal conditions: two rela schemas in one database — the standard
schema-per-project layout, and what the conformance harness already creates —
with one holding either lock. Every starting process is a migrator
(`pgstore.Open` calls `Migrate`), so migrate contention needs only two processes
starting; write contention needs only two schemas taking writes.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded as why1–why5 on the bug. Two systemic findings worth restating:

**The codebase had already solved this twice.** The change feed schema-qualifies
its NOTIFY channel (`rela_changed_<schema>`) because LISTEN/NOTIFY is
database-global, and `feed.go` documents that reasoning. PR #1217 then applied
the same reasoning to the version-sweep lock. Neither pass generalised to the
remaining two locks, so this is the third instance of one insight.

**Severity gradient hid the tail.** The sweep's variant caused *silent data
loss* (non-blocking acquire, `return nil` on failure, capture dropped with no
error) and surfaced when e2e workers collided. The migrate and write variants
merely *block* — correct but slow — so nothing forced them into view. The
low-severity instances of a defect outlive the high-severity one.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Approach: adopt the two-key form #1217 established —
`pg_advisory_xact_lock($1::int, hashtext(current_schema()))` — rather than a
competing mechanism. One idiom across all three locks, Postgres-native, no
Go-side schema resolution and no extra round-trip.

An earlier draft of this fix used a Go-side key derivation
(`class<<32 | hash(schema)`, resolved via `SELECT current_schema()` per call).
It was abandoned on rebase: #1217's SQL-level approach is simpler, and the Go
version had generated its own defects during review (a NULL-vs-empty scan bug, a
JOIN that returned no rows instead of NULL, and a migrate bootstrap split needed
because the table-anchored variant fails on a fresh database). Thoroughness that
manufactures its own bugs is not an improvement.

Regression test: must drive the **real** code paths and hold **both** the scoped
and legacy key spaces — see the measure entity for why the obvious version does
not falsify.

Related areas checked:

- `sweepAdvisoryLockKey` — already fixed by #1217; left alone.
- `migrateAdvisoryLockKey`, `writeAdvisoryLockKey` — fixed here.
- NOTIFY channel — already schema-qualified.
- Sequences (`rela_seq`, `version_seq`), per-schema pools — per-schema by nature.

One adjacent gap left open deliberately: `resolveChannel` (`feed.go`) has an
unreachable `schema = "public"` fallback, because `current_schema()` returns
NULL rather than an empty string. Pre-existing, harmless at startup-time channel
resolution, and out of scope here — worth its own ticket.
