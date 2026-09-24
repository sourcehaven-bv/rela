---
id: PLAN-W7T3WE
type: planning-checklist
title: 'Planning: sqlitestore: push GraphQuery down into SQL like pgstore'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- `sqlitestore` renders `store.GraphQuery` as SQL for `GraphQuery`,
`GraphCount`, `MatchingIDs`, and the optional `MatchedCounter` (`CountMatched`),
`GraphHeaderQueryer` (`GraphQueryHeaders`) and `HeaderReader`
(`ListEntityHeaders`). Covered: props (all `PropOp`s, scalar guard, list
containment), `HasInbound`/`HasOutbound` with inheritance closures bounded by
`graphquerynaive.DepthCap`, `EndpointMatch` chains, `Related`, `Any`,
`Narrowing`, world scoping, `FaceIn`, ordering (including enum `Values`),
paging.
- Derived indexes: `sqlitestore` implements `Reconcile` for
`DerivedQueryIndex` and `DerivedListIndex` (all-or-nothing desired state, owned
name prefix only), wired in appbuild and `rela db` for the sqlite build.

Out of scope:

- A shared SQL builder package for pg and sqlite (follow-up once both are
stable; see Alternatives).
- `pgstore/visiblesearch.go` equivalent: sqlite search is bleve plus
`search.NewVisible`, which already calls `GraphQuery`/`MatchingIDs`.
- `DerivedUnique`: uniqueness stays application-level on sqlite.
- Changing pgstore behaviour.

**Acceptance Criteria:**

1. `storetest.RunAll` passes on sqlite through the SQL path (GraphQuery,
paging, headers, EndpointMatch, Related, Worlds, VisibleSearch).
2. A differential test compares the SQL path with `graphquerynaive` on the
same seeded sqlite store, over generated queries and a value mix.
3. `EXPLAIN QUERY PLAN` tests show the derived query and list indexes are used,
mirroring the pg EXPLAIN tests.
4. A dataentry budget test on sqlite shows the same statement count at 10 and
50 rows for a list page and a pushed traversal scope.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach mirrors an existing in-tree backend, pgstore; no open design question needs a survey)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- pgstore `graphquery.go` (builder: `buildPredicateParts` 223,
`buildPredicateSQL` 632, `nestedPredicateSQL` 785, `buildGraphQuerySQLSelect`
476, `orderKeySQL` 590) and `derivedschema.go` (reconcile, index naming,
`orderRankSQL` 161) are the reference.
- `sqlitestore/world.go` `worldSQL` already mirrors pg's world ranking;
`entityquery.go` primes one row per id with `ROW_NUMBER()`.
- `graphquerynaive` is the behavioural reference and supplies
`CheckEndpointShape` and `DepthCap`.
- No library: query builders (squirrel, goqu) do not cover recursive closures
or JSON dialects any better than hand-built SQL, and pgstore is hand-built.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

A separate builder in `sqlitestore/graphsql.go` mirroring pg function by
function, with the same names.

- Placeholders are numbered `?N` (pg appends args out of textual order).
Lists bind as one JSON array: `IN (SELECT value FROM json_each(?N))`.
- JSON paths are SQL literals, never bound, so a query expression matches its
index expression. `jsonPath(prop)` renders `'$."prop"'` with `'` doubled and
refuses names containing `"` or control characters; a refused name sends the
WHOLE query to `graphquerynaive` (never one arm: a `NOT` around a skipped arm
would invert it).
- Text form: `txt(alias, path)` = `CASE json_type(..) WHEN 'true' THEN 'true'
WHEN 'false' THEN 'false' ELSE CAST(.. ->> path AS TEXT) END`, matching
`propmatch.Stringify` for strings, integers and booleans. Scalar equality is
`json_type(..)='text' AND .. ->> path = ?N` so it can use an index. Containment
is `EXISTS (SELECT 1 FROM json_each(.., path) WHERE type='text' AND value =
?N)`.
- Relations: recursive CTEs ported from pg, bounded by `DepthCap`; endpoint
hops pinned to the default face as pg does.
- Worlds: predicates and scope in an inner query with `ROW_NUMBER() OVER
(PARTITION BY e.id ORDER BY rank, e.face)`, outer `rn = 1`, then order and page.
Counts use `count(DISTINCT e.id)`.
- Ordering: `(key IS NULL), key` per spec, so absent sorts last ascending as
in pg and naive; enum `Values` via one `CASE` helper shared by DDL and query;
`e.id` tiebreak. Default BINARY collation equals Go byte order.
- Every statement goes through `s.q()` so it sees writes in the same `Tx`.
- Phases: (1) props/order/paging/counts/headers, with relation predicates on
the naive path; (2) relation predicates, removing that fallback; (3) derived
indexes, wiring and EXPLAIN tests.
- Derived indexes: `sqlitestore/derivedschema.go` lists owned indexes via
`sqlite_schema` with `GLOB 'rela_derived_*'`, diffs against desired specs,
creates/drops under `writeMu` in one `BEGIN IMMEDIATE`. Index names reuse pg's
hashing. Leading column `type`, guard terms `face = ''` and
`json_type(..)='text'` written identically in DDL and query. Spec loading in
appbuild is moved to a build-agnostic helper; `derivedschema_nosweep.go` becomes
`!postgres && !sqlite`.

Alternative rejected: a shared dialect package. Almost every leaf (placeholders,
arrays, text form, type test, containment, collation, null order, world priming,
literal paths) would need a hook, and pg's SQL spellings are pinned by EXPLAIN
tests; routing pg through an abstraction risks silently losing an index. The
differential test keeps the two builders honest instead.

**Files to modify:**

- `internal/store/sqlitestore/graphquery.go`, new `graphsql.go`,
`derivedschema.go`, `world.go` (numbered placeholders), `sqlitestore.go`
(plimsoll counts), tests.
- `internal/store/storetest/` new differential harness.
- `internal/appbuild/derivedschema_*.go`, `internal/cli/db_sqlite.go`.
- `internal/dataentry/querybudget_test.go` (+ sqlite variant),
`listpushdown_scope_sqlite_test.go`.
- CLAUDE.md (derived-index bullet, sqlitestore paragraph), docs on the sqlite
backend.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Property names come from requests (filters, sort) and config. They are
rendered as SQL literals in JSON paths, so `jsonPath` is the injection boundary:
`'` doubled, `"` and control characters refused (whole query falls back to
naive). Fuzzed.
- Values, types, ids and relation types are always bound parameters.
- Derived index DDL uses validated names only; unsafe names are refused and
reported, as in pgstore.

**Security-Sensitive Operations:**

- The ACL read gate arrives as `GraphQuery` fields; the SQL path must AND
every field. The storetest Related/visible-search suites and the dataentry
pushed-vs-Go comparison on sqlite guard against widening.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: `TestConformance` on sqlite (already runs `RunAll`).
2. AC2: `storetest.RunGraphDifferential(t, factory)`: seed one store, compare
`GraphQuery`, `GraphCount`, `CountMatched`, `MatchingIDs` and headers with
`graphquerynaive` on the same store, over a seeded generator plus a fuzz target.
3. AC3: `sqlitestore/graphquery_explain_test.go` mirroring the seven pg EXPLAIN
tests; assert `USING INDEX rela_derived_...` and no `USE TEMP B-TREE FOR ORDER
BY` on list pages.
4. AC4: split `newBudgetApp` over a base store; sqlite variant asserts equal
statement counts at 10 and 50 rows via a statement counter exported for tests
(the `Counting` wrapper cannot see naive's inner reads).
5. `listpushdown_scope_sqlite_test.go` runs the pushed-vs-Go comparison.
6. Reconcile lifecycle: create, idempotent, drop orphans, dry-run, unsafe name.

**Edge Cases:**

- Values: string, `""`, int, float, bool, list, empty list, JSON null, absent.
- Property names with `'`, `"`, `.`, `[`, unicode, spaces.
- Inheritance chains longer than `DepthCap`; cycles.
- Faces: default-only rows, face-only rows, `FaceIn`, world fallbacks.
- Paging past the end, `Limit` 0, ties on sort keys.
- Queries inside `Tx` see uncommitted rows.

**Negative Tests:**

- Nested inheritance in an endpoint is refused with the naive error.
- Unsafe property names fall back to naive and still answer correctly.
- Reconcile refuses an unsafe spec and leaves unowned indexes alone.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Text-form drift from naive for floats: keep floats out of equality claims or
match `fmt.Sprint`; the differential test covers it.
- Planner choosing `entities_type_idx` without statistics: run `PRAGMA
optimize` after reconcile; EXPLAIN tests pin the choice.
- Expression drift between DDL and query: one helper per expression, verbatim
tests.
- Arg-order bugs: numbered placeholders plus a binding test.
- `CREATE INDEX` at boot blocks writers on large databases: same trade as pg,
documented.
- Effort: xl.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- CLAUDE.md: derived-index bullet (no longer postgres-only), sqlitestore
paragraph.
- The sqlite backend guide and `rela db` reference: reconcile now does work.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- Critical, addressed: RR-7UAK0R.
- Significant, addressed: RR-LUJRQ7, RR-3UTGG1, RR-O88GD0, RR-4H2WCG,
RR-Y2CE1I.
- Minor, addressed: RR-RIFNA8, RR-ZL76L1, RR-Y3FXUP, RR-6S12DF, RR-BOZBFA,
RR-BF7WIT, RR-HR17TV, RR-6XTXAW, RR-YIP0EQ, RR-KHO9NV, RR-H2S268, RR-Z8XY3X,
RR-O8WM4Y.
- Nit, addressed: RR-GJJE7A, RR-4UHR03.
