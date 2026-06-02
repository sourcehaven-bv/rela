---
id: RR-U9RFH
type: review-response
title: Conformance factory is called ~100x — each needs an isolated empty store
finding: 'storetest.RunAll invokes factory(t) / searchFactory(t) ~100 times (one per subtest: 23 entity + 26 relation + 11 query + 13 pagination + 11 watcher + 14 attachment + 3 validation + ~21 search). Every call MUST return a fresh, EMPTY store (storetest.go:3,18); tests assert empty-state behavior directly (GetNotFound, HighestID(''NOPE'')==0). Against a single shared PostgreSQL, leaked rows between subtests cause non-deterministic failures — the biggest correctness risk in the plan. The plan only said ''isolated schema / truncate'' as an aside without committing to a mechanism or accounting for ~100 cycles.'
severity: critical
status: open
---

## Finding

`storetest.RunAll` invokes `factory(t)` / `searchFactory(t)` **~100 times** (one
per subtest). Every call MUST return a **fresh, empty** store
(`storetest.go:3,18`); tests assert empty-state behavior directly. Against a
single shared PostgreSQL, leaked rows between subtests cause non-deterministic
failures.

## Resolution (DECIDED)

**Per-schema isolation, enabled by connection injection (see RR-PG-INJECT).**

`pgstore.New(db DBTX, ...)` accepts a pgx handle rather than owning the pool.
The conformance factory then:

1. Holds ONE shared `*pgxpool.Pool` for the whole package (built once).
2. Per factory call: `CREATE SCHEMA test_<sanitized-name>_<counter>`; configure
a pool view whose `BeforeAcquire`/`AfterConnect` runs `SET search_path TO
<schema>` so every store query lands in that schema without the store knowing
about schemas at all.
3. Run embedded migrations into THAT schema (the `rela_seq` sequence is
per-schema, so each test gets a fresh seq — better isolation than TRUNCATE).
4. `pgstore.New(scopedHandle)` + `t.Cleanup(DROP SCHEMA <schema> CASCADE)`.
5. `searchFactory` wires the pg search backend against the SAME scoped handle.

This keeps the **production code path identical under test** — real pool, real
commits, real internal transactions for rename/cascade — so `FuzzConcurrentOps`
(+ `-race`) and the after-commit watcher semantics behave exactly as in prod. It
is also parallel-safe (distinct schemas don't contend).

**Perf:** ~100 `CREATE SCHEMA` + empty-table migrate cycles is expected to be a
few seconds (milliseconds each). Measure first; only if slow, optimize via a
migrated template schema cloned per test, or fall back to TRUNCATE + `ALTER
SEQUENCE rela_seq RESTART` in one shared schema (serial subtests).

Rejected alternative: **transaction-rollback isolation.** It conflicts with this
contract — the store needs a pool (not a single pinned tx) for concurrency, the
watcher emits AFTER commit (rollback never commits), and the store's own
rename/cascade transactions would be forced into savepoints coupling the harness
to store internals. Per-schema is strictly better here.
