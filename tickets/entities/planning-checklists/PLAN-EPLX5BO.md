---
id: PLAN-EPLX5BO
type: planning-checklist
title: 'Planning: Page executeQuery and the remaining scopedHeaders callers'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the structured `/_search` branch (no free text) reads one gated, ordered,
limited page per type in the store and counts with `store.CountMatched` only
when the page is full; the `_position` search scope over one type is answered by
`store.GraphPosition`; the search budget tests are converted to rows checks.

Out (left to other work): next action and `pick_one` (TKT-CKLZMWB), dashboard
count/breakdown cards (TKT-AIEGHU) and table cards (TKT-8AUD1U), nav status
counts, the whole-set callers' own read functions (export, feed, gantt, command
payload, which TKT-F2EJQY8 also touches), and deleting
`TestUnpagedReadAllowlist_ScopedHeadersIsDebt`. Follow-ups are filed.

**Acceptance Criteria:**

1. A structured search the plan takes returns the same rows, order and total
as the Go path, for an allow-all reader and a role-relation reader
(`TestSearchPlan_MatchesGoPath`).
2. The search scope position agrees with the Go path
(`TestSearchPlan_PositionMatchesGoPath`).
3. A sorted, limited structured search reads a bounded number of rows at
every store size (`TestQueryBudget_SearchLimitIsSizeIndependent`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach reuses the list pushdown pattern)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal query planning)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal query planning)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `planListPushdown` (`internal/dataentry/listpushdown.go`): exact lowering
of list filters and sorts, then a bounded page and a count.
- `queryplan.StringShaped`, `queryplan.DeclaredValues`: the sort keys the
store orders like `filter.QuerySort` (TKT-9OFGH4).
- `scopedHeaders`: the one verdict switch, now split into `scopedQuery` the
same way TKT-F2EJQY8 splits it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`planSearch` accepts a structured query only when every filter is `=` or `!=`
(non-empty) on a scalar string or declared-enum property on every named type,
and a sort only on one type with string-shaped, declared, non-rollup keys.
Anything else takes the old Go path. Each readable type's query comes from
`scopedQuery`, so the ACL verdict, world and face allowlist are inside the
query. Alternative rejected: a generic Go fallback per row, which keeps the
scan.

**Files to modify:** `internal/dataentry/searchplan.go` (new),
`queryservice.go`, `scopedread.go`, `api_v1.go`, `scope.go`, budget and search
tests, `internal/archguard/unpagedread_allowlist_test.go`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The `q`, `type` and `limit` parameters of `/_search` and the scope descriptor of
`_position`. The plan is an allowlist of operators and property shapes;
everything else declines to the existing path. `limit` keeps its 1..100
validation.

**Security-Sensitive Operations:**

Row gating: every query is built by `scopedQuery` from the principal's verdict,
and the total is counted on the same gated query, so it is a post-gate count.
Field-level `visible:` on filters and sort is unchanged (BUG-BINAR2 tracks it
for both paths).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** the differential tests over 27 queries, two principals and
three limits; the budget tests; the existing `/_search` and `_position` handler
tests.

**Edge Cases:** empty and missing values, `!=` on an empty value, undeclared
enum values, custom enum types, the legacy `status` type, dates, integers,
lists, repeated and undeclared types, an exact type outside the query's types.

**Negative Tests:** declined queries still run and must match the Go path.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** the store and Go could judge a value differently; mitigated by the
narrow allowlist and the differential test. Effort: l (first part done here).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `meta.total` of `/_search` now counts every match;
recorded in the ticket and the PR body.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design copies the reviewed list pushdown; the code review covers it)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none
