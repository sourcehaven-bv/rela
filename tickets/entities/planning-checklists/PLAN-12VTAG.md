---
id: PLAN-12VTAG
type: planning-checklist
title: 'Planning: List export reads relation columns in one batched query'
started: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: the built-in table renderer of the list export
(`resolveListRelations` / `gatherListPeers` in
`internal/dataentry/export_list.go`), which read each row's edges with one or
two queries per row. The seqtrace trace of `GET /tasks/_export` shows
`1100× ListRelations` for 550 rows; the other 7 queries are fixed.

Out of scope: the neighbor gate and title resolution (already one batch each),
the `export_render:` script path, entity export, and any change to what the
export shows or to ACL/visibility semantics.

**Acceptance Criteria:**

1. The list export issues a fixed number of store reads, independent of the
   row count. Test: a `storetest.Counting` budget test at 10 and 50 rows,
   under the fixture ACL and the realistic ACL.
2. The export output is unchanged. Test: the existing `TestExport_List_*`
   tests pass, plus a test that compares the batched output with the per-row
   output on a fixture with both directions, shared neighbors, an edge between
   two exported rows and a hidden neighbor.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change that reuses an existing batched read)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal query batching, no library applies)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the reference implementation is the list API in this repo)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `entityReader.pageRelations` (`internal/dataentry/entityreader.go`): one
  `RelationQuery{EntityIDs, DirectionBoth}` for a page, split per row. The list
  API's nil-worldNeighbors arm (`servedFacePageEdges`) uses it (TKT-1U8XYN).
- `.claude/rules/collection-reads.md`: one `RelationQuery.EntityIDs` query per
  page; pin the cost with a budget test.
- `querybudget_test.go`: `readsFor` / `assertBudget` harness.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`gatherListPeers` calls `h.reader.pageRelations` once over the distinct row
ids, keys the result by id, and then applies the same per-row steps as before:
`edgesOwnedBy` / `incomingOwnedAtZero` with the row's face, then
`relationPeers` per column. The store returns relations in a stable key order
on every backend, and the batched query returns a superset filtered by the
same predicate, so each row's edge order, and therefore the cell order, is the
same as with the per-row queries. Rows are deduplicated by id before the call
because `pageRelations` gives a repeated id no edges; the export can serve two
faces of one id.

**Alternatives rejected:**

- Switch the export to the list API's world-aware `worldNeighborsForPage`:
  that changes which neighbor face supplies the title and how neighbors are
  gated, so it changes what the export shows. Out of scope.
- One query per relation column (like view sections): more queries than one
  `DirectionBoth` query and no benefit here.

**Files to modify:**

- `internal/dataentry/export_list.go`
- `internal/dataentry/export_budget_test.go` (new)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

No new input. The row set is the ACL-scoped, filtered set the handler already
computes; the columns come from operator config.

**Security-Sensitive Operations:**

The edge read is raw, as before; neighbor visibility is still decided by the
single `visibleRelationIDs` gate and titles are still derived after
`visibility.Redact`. A store error is logged and truncates the edges, the same
degraded behavior as the per-row read.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. Budget: `TestQueryBudget_ListExportIsSizeIndependent` drives the HTTP
   handler over the budget fixture (two relation columns) at 10 and 50 rows.
2. Output: existing `TestExport_List_*` tests, and a comparison test for the
   edge cases below.

**Edge Cases:**

- Incoming and outgoing columns on the same relation type.
- An edge between two exported rows (outgoing for one, incoming for the other).
- A neighbor shared by several rows.
- No relation columns: no edge query at all.
- Empty export: no edge query.

**Negative Tests:**

- A hidden neighbor stays out of the cell (`TestExport_List_HiddenNeighborExcluded`).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Cell order could change if a backend did not keep a stable order. All four
  backends order by relation key (pg/sqlite `ORDER BY`, fs/mem a global key
  order), and the comparison test pins it.
- One query with up to `listExportCap` (5000) ids: pgstore binds an array,
  sqlitestore a JSON array, so there is no placeholder limit.

Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal refactor, no behavior change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor ticket, not an enhancement)

**Documentation Impact:**

N/A. Internal change, no user-facing docs needed.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: a mechanical swap to an existing batched read with no design choice; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
