---
id: PLAN-DDZ5I1Y
type: planning-checklist
title: 'Planning: Next-action suggestion grows with the data on postgres'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: the candidate read of a next-action query source on postgres, and
the derived query index it reads. Out of scope: other executeQuery callers
and the remaining scopedHeaders callers (TKT-D3RAB0E), and the gated count of
`countIsZero` (follow-up).

**Acceptance Criteria:**

1. A query source reads a bounded window of candidates, not every match.
   Test: `TestNextActionBudget_QuerySourceReadsAWindow` (scale-free reads).
2. The suggestion on screen stays when it falls past the window. Test:
   `TestNextAction_CurrentPastTheWindowStays`.
3. Rows dropped by the Go filter do not hide later matches. Test:
   `TestNextAction_WindowSkipsRowsTheGoFilterDrops`.
4. A window over a static query reads the derived index in id order. Test:
   `TestGraphQueryExplainWindowReadsInIndexOrder`.
5. The postgres scale run ratio for next-action improves.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the cause was found by measurement; one clear fix)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal query path)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

`store.GraphQuery` already has Limit and Offset; pgstore renders them as one
`ORDER BY id LIMIT` statement. The scale-free budget harness in
`internal/dataentry` (`readsFor`, `assertBudget`) pins the read count.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Measured with tools/scalerun and EXPLAIN: each resolve read all critical
tasks in keyset pages of 500, and each page sorted the whole match because
the derived index had no id column. The engine then kept 20.

1. The engine tells the candidate func how many leading candidates it keeps
   (`nextaction.Want`), 0 when a condition filters after the fetch.
2. dataentry reads windows of the match per type in id order, doubling up
   to 500, and stops once the Go filter has kept enough. When the
   suggestion on screen is past the window it falls back to a full read.
3. The derived query index gets id as its last key column, so a window
   reads the index in order and stops.

Rejected: pushing ordered comparisons into the store (date strings with a
time zone break the superset guarantee); caching candidates across requests
(per-principal, and stale).

**Files to modify:** internal/nextaction/nextaction.go,
internal/dataentry/{nextaction,queryservice,helpers,scopedread}.go,
internal/store/pgstore/derivedschema.go, tests, tools/scalerun/scalerun.yaml,
the scale-run guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The query is operator config; `current_entity` comes from the request and is
only compared against ids the gated read returned.

**Security-Sensitive Operations:**

Row gating stays in the store query: the window is applied to the gated
GraphQuery, and the Go filter still runs on every returned row. A window is
refused when a `Scope` is set, so a scoped match query is never truncated.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria.

**Edge Cases:** current suggestion past the window; a window with no row
that passes the Go filter; a source with a condition (no window); a
multi-type query; a sorted query (no window).

**Negative Tests:** a window with a Scope is rejected with an error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** the candidate set changes from all matches to the first matches in
id order; the engine already truncated to the first 20 in that order, so the
pick is unchanged. Renaming the derived index rebuilds it once on upgrade.
Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** the postgres scale-run guide (ratio limit).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small performance change; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
