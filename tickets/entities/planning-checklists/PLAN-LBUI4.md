---
id: PLAN-LBUI4
type: planning-checklist
title: 'Planning: Add Tx write-transaction contract to store.Store with fs/mem/pg implementations (DEC-8UIL0 phase 1)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN: `Tx(ctx, fn func(Store) error)` on `store.Store` (embedded
`Transactor` interface); fsstore/memstore mutex implementations with tx view
(reduced single-user guarantees: no rollback, inline events); pgstore native
transaction
+ global advisory lock + post-commit event buffering + rollback; storetest
conformance; root CLAUDE.md rule amendment per [[DEC-8UIL0]]. OUT: entitymanager
intent wrapping, Lua `rela.tx` helper, writeMu deletion, fs rollback journal,
per-entity lock granularity, retry-on-conflict (all later slices of the
DEC-8UIL0 arc).

**Acceptance Criteria:**
1. All three backends compile as `store.Store` with `Tx` — conformance suites pass (default, memorybackend, postgres tags).
2. Writes made through the Tx view are visible after Tx returns (storetest `WriteVisibleAfterTx`).
3. N concurrent `Tx(read-modify-write)` calls lose no updates (storetest serialized-counter test, race detector on).
4. A write inside `fn` does not deadlock; nested `Tx` joins (storetest re-entrancy/nesting tests).
5. `fn` error propagates out of `Tx`; on pgstore the write is rolled back and no in-process events fire (pg-only `RunTxRollbackTests`).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: covered by RES-Z1SJ5 option survey)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** [[RES-Z1SJ5]]

**Existing Solutions:**
- pgstore already wraps every write in a tx for pg_notify atomicity (entity.go:213, feed.go) and fans out in-process events post-commit — Tx generalizes this.
- Advisory-lock idiom established: migrate.go:56 (xact), sweep.go:19/purge.go:49 (session).
- `DBTX` seam (pgstore.go:56) already admits `pgx.Tx`; `Begin` on a tx = savepoint, so per-write methods run unchanged inside an outer Tx.
- fsstore/memstore single-RWMutex concurrency documented in package docs; tx view + separate txMu follows the same structure.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Per [[DEC-8UIL0]] (options A–E analyzed and rejected in
[[RES-Z1SJ5]]): interface member (not optional capability — every backend
implements, callers would otherwise need a fallback path). fs/mem: new `txMu`
mutex; public write methods take it briefly; `Tx` holds it across `fn` and
passes a `txStore` view whose write methods call the unexported cores
(re-entrancy by structure). pgstore: `Tx` opens the outer tx, takes
`pg_advisory_xact_lock` on a new write key, hands a tx-bound `*Store` view (db =
outer tx; savepoints per write); notifyPut/notifyDelete/emit/emitAll defer to a
buffer replayed after commit. Nested Tx joins (view.Tx calls fn directly /
txPending short-circuit).

**Files to modify:** `internal/store/store.go`;
`internal/store/fsstore/{fsstore,entity,relation,attachment,tx}.go`;
`internal/store/memstore/{memstore,tx}.go`;
`internal/store/pgstore/{pgstore,entity,tx}.go`;
`internal/store/storetest/{storetest,tx}.go`; pg conformance test; root
`CLAUDE.md`; `docs/postgres-backend.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input surface — `Tx` takes a callback, no
user data. Existing per-write validation (validateID etc.) unchanged inside the
view.

**Security-Sensitive Operations:** Advisory lock key is a compile-time constant
(no user influence). Denial-of-write via a long-held Tx is bounded by callers'
existing timeouts; no Tx callers are introduced in this PR. No auth/ACL changes
(ACL sits above the store).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1 → conformance suites (fs/mem run in default CI; pg via
`just test-postgres`). AC2 → `RunTxTests/WriteVisibleAfterTx`. AC3 →
`RunTxTests/SerializedReadModifyWrite` (8 goroutines, counter property, race
detector). AC4 → `RunTxTests/ReadYourWrites`
+ `NestedTxJoins`. AC5 → `RunTxTests/ErrorPropagates` + pg-only `RunTxRollbackTests`
(entity absent after failed Tx; no buffered events delivered).

**Edge Cases:**
- Write inside fn via the view (must not deadlock) — covered.
- Nested Tx — joins, covered.
- fn error mid-sequence: fs keeps earlier writes (documented reduced guarantee, not asserted); pg rolls back (asserted).
- Concurrent ordinary writes during an open Tx: serialized via txMu / advisory lock (counter test).
- Subscriber events during pg Tx: delivered only post-commit; none on rollback (asserted in rollback suite).

**Negative Tests:** fn returns error → Tx returns that exact error (errors.Is);
pg: no entity, no events.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- fs deadlock via wrong lock path → structure (wrappers vs cores) + conformance re-entrancy test + race detector.
- pg event buffering drops/duplicates → buffer replay only after successful commit; rollback suite asserts silence.
- Interface growth breaks external implementers → grep confirmed only the three in-tree backends implement store.Store.
- plimsoll pins on store impls → bump with required-interface justification (documented exception category).
Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] CLAUDE.md - amend "no transaction abstractions" rule per DEC-8UIL0
- [x] docs/postgres-backend.md - short "write transactions" section
- Interface godoc on store.Transactor carries the contract.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: design settled by RES-Z1SJ5 five-option analysis + DEC-8UIL0, accepted by owner 2026-07-17 with explicit fs-guarantee reduction; sub-decisions (interface member, joining detection, lock granularity) recorded in the decision)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none open — see [[DEC-8UIL0]]
