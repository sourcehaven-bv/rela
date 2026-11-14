---
id: PLAN-K2ICLL1
type: planning-checklist
title: 'Planning: Gantt read grows faster than the data on postgres'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the faster-than-linear growth of the gantt root view on postgres.
A local run (scale 1 and 3, `-verbose` statement log, pprof) showed its cause in
pgstore, not in the gantt: the edge read sends every node id twice (`EntityIDs`,
`PeerIDs`), the iterator re-sends both arrays on every 500-row page, and once
PostgreSQL switches the cached statement to a generic plan, `to_id = ANY($3)`
compares every row with every id. Both are quadratic. Out: making the root view
grow with the rows it shows. That needs the roll-up in the store; follow-up
TKT-EBTVUL8.

**Acceptance Criteria:**
1. A pgstore id-batch read (ListRelations with EntityIDs/PeerIDs, entity
listings with IDs) is one statement when the result fits the batch.
2. A large id batch is never run from a cached prepared statement.
3. The gantt edge read is one statement per relation type on postgres at any
chart size.
4. Gantt output and its ACL order are unchanged.
5. Scale ratio of the gantt endpoint drops from 11x to about linear.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the cause was measured directly)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: pgx already offers per-query exec modes)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: standard PostgreSQL generic-plan behavior)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `pgstore/pagedseq.go` documents why iterators page (BUG-9TGOH1); the fix
keeps paging and only sizes the page.
- `pgstore/graphquery.go` (TKT-9OFGH4) already records the generic-plan trap
for expression indexes.
- `store.WithQueryStats` counts statements in tests.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `queryPage` takes the size of the id batch the statement
carries. The page is at least that large, so the ids cross the wire about once.
Above 500 ids the statement runs with `pgx.QueryExecModeDescribeExec` (unnamed),
so PostgreSQL plans it for the actual arrays and hashes `= ANY`.

Rejected: `IN (SELECT unnest(...))` (generic plans still pick nested loops, 35 s
measured); dropping `PeerIDs` in the gantt (reads edges outside the chart, which
TestGanttEdges_CostIsScaleFree forbids); a single unpaged `ListRelationsPage` in
the gantt (fixes one caller, not the store, and still hits the generic plan);
`plan_cache_mode` per connection (affects every statement).

**Files to modify:**
internal/store/pgstore/{pagedseq,entity,relation,graphquery}.go, new tests in
pgstore and dataentry, tools/scalerun/scalerun.yaml, the scale-run guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new input. The ids still go as bind
parameters; only the exec mode and the LIMIT (an integer) change.

**Security-Sensitive Operations:** None. The gantt's ACL order (gate, redact,
fold, cap) is untouched.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. `TestListRelations_IDBatchReadsInOneStatement` (page size 10, 30 and 120
edges, one statement each).
2. `TestListRelations_LargeBatchIsNotPrepared` (one-connection pool,
pg_prepared_statements unchanged after 600 ids, grows after 20).
3. `TestGanttEdges_OneStatementPerRelationType_Postgres` (100 and 1200 edges).
4. Existing gantt tests, golden parity (default, sqlite), pgstore suite.
5. Local measurement at scale 1 and 5.

**Edge Cases:** Empty batch (non-nil, matches nothing) keeps its meaning; a
result larger than the batch still pages; nil batch keeps the 500-row page.

**Negative Tests:** Both pgstore tests fail with the change reverted (verified).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Custom planning costs a few ms per thousand ids: measured 80 ms at 68k ids,
far below the generic-plan cost.
- A page as large as the batch holds more rows at once: bounded by the ids
the caller already holds. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] Scale-run guide: gantt row of the measured table.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: store-internal paging change; reviewed with the code by both review agents)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: see above)

**Design Review Findings:** N/A
