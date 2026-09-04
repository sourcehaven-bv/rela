---
id: IMPL-BCZLPW
type: implementation-checklist
title: 'Implementation: PostgreSQL read-path performance audit: per-request query counting, seeded demo with ACL + worlds, slow-query driven optimisation'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Tests added: `store/querystats_test.go`, `pgstore/tracer_test.go` +
`tracer_pool_test.go` (DB-gated stats incl. Tx),
`dataentry/requeststats_test.go` (header under Debug only, SSE excluded, router
wiring), `perfseed_test.go` (determinism, stream independence, shape,
referential integrity, load into memstore, cancel), storetest conformance for
`RelationQuery.EntityIDs` (union-of-scalar-calls, nil vs empty, composition),
`GraphQueryHeaders`, `OrderBy/Limit/Offset` (missing-largest ordering, window =
slice, count ignores paging, headers page the same), `CountMatched`;
`acl/requestreuse_test.go` (one walk per bound request, foreign request
ignored); `dataentry/querybudget_test.go` (size independence + pinned
constants), `rowcontent_test.go` (content omitted/opt-in on list and search,
ordering + total intact), `listpushdown_test.go` (differential pushed-vs-Go for
8 shapes × 2 page sizes × 3 pages, eligibility table, scoped total for a gated
principal), `views_test` recursive traversal (set semantics), `queryplan`
list-index specs, `pgstore` list index naming/DDL, EXPLAIN uses the list index
with no Sort node, ranking prefers a title match, migration 0014 rebuild equals
the store's composition, `userstatetest` batch lookups on mem/kv/pg.

Edge cases from planning: dangling neighbour ids (absent from header batch →
dropped, as before); duplicated ids per page; empty `EntityIDs` matches nothing
with an explicit `::text[]` cast on pg and `(NULL)` on sqlite; page beyond the
end (clamped window, correct total); sort key absent on some rows (largest-value
semantics on both paths, pinned by the differential test); errored statements
still counted; seeder at scale 0.01 and `--force`; foreign/mismatched
`acl.Request` on ctx not reused.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1: `curl -D - .../api/v1/tasks` against `rela-server-postgres -verbose` shows `Server-Timing: db;dur=3.1;desc="8 queries"`; without `-verbose` the header is absent; the log carries one `msg=request` line per request.
- AC2: `go test -run QueryBudget ./internal/dataentry/` — list 6, view section 11, search 3 store calls at both 10 and 50 rows.
- AC3: `rela-postgres dev seed --project prototypes/perf/project --scale 1` → 19,117 rows + 47,549 relations in ~40 s; second run refuses ("store already holds 18,000 entities"); `.rela/audit.jsonl` carries a `perf-seed` record on the fs build; `rela analyze all` on a seeded fs copy reports no property/cardinality issues.
- AC4: table in the ticket body (`.ignored/perf/baseline-before.tsv` vs `after-d4.tsv`, second pass): every listed endpoint's query count is now independent of rows; DB time drops 5–100× on lists, kanban, search, next actions; reader/editor rows match.
- AC5: `TestGraphQueryExplainPagedListUsesDerivedListIndex` (index scan, no Sort); startup log on the perf DB shows two `rela_derived_list__` indexes created; `pg_indexes` confirms `entities_type_id_idx`, `entities_id_prefix_idx`, tsv index gone.
- AC6: existing ACL list/view/search tests pass over batched paths; `TestListPushdown_ScopedTotalForGatedPrincipal` (total = visible count, denied type empty); world tests in `dataentry` and `storetest` pass with the faceless-type collapse (byte-identical results).
- AC7: `TestListRows_OmitContentUnlessRequested`, `TestSearchRows_OmitContentUnlessRequested`; SPA components read no list-row content (grep); e2e run recorded below.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — `queryplan.StringShaped` is the one definition of "byte-comparable property" shared by prefilter pushdown, list pushdown and index derivation; `rowKey`/`loadRows` shared by content reload and hit loading; `edgesPerIndex`/`each` in perfseed
- [x] No security issues introduced (Server-Timing gated on Debug; scoped counts only; batched reads go through the same gates; seeder is attributed and audited)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
