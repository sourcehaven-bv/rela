---
id: PLAN-ST0VGF
type: planning-checklist
title: 'Planning: Gantt perf: header projection + SQL-scoped subtree drill'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** see the ticket body. In: header-projected forest load; GraphQuery
subtree drill; verdict-gated fallbacks. Out: forest cache (option C),
where:-pushdown.

**Acceptance Criteria:**

1. Full build loads via `ListEntityHeaders` under an AllowAll verdict; the
   drilled response is byte-identical to the same subtree of the full build.
   *Test:* `TestGantt_SubtreeDrillMatchesFullBuild` (json-equal + fold-through-
   closure + breach survival).
2. All existing gantt ACs still hold on the new paths. *Test:* the full
   TKT-MW28U5 suite unchanged (ACL differentials now exercise the fallback).
3. Measurable improvement on the postgres harness. *Evidence:* REPORT.md
   after-table — full 1.8× at 50k, drill 6×.
4. Where-excluded / denied / missing drill roots stay uniform 404s.
   *Tests:* `TestGantt_SubtreeDrillWhereExcludedRoot`,
   `TestGantt_DrillRootUniform404` (unchanged, now via fast path).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** the measured scaling report (`.ignored/gantt-perf/REPORT.md`)
IS the research: 4 scales on real postgres, CPU-profile attribution.

**Existing Solutions:** `store.ListEntityHeaders` (store.go:471, pgstore
implements HeaderReader) + `visibility.RedactHeader` — both existed;
`store.GraphQuery` with `RelationPredicate.InheritThrough` endpoint closure
(the ACL pushdown seam, EXPLAIN-tested) expresses "descendants of root" in
SQL. No new store surface.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** per-type verdict switch in `loadGanttType`
(DenyAll→skip, AllowAll→headers, scoped Query→full-entity fallback);
`buildGanttSubtree` fast path gated on all-AllowAll verdicts with
`finishGanttForest` shared so the two paths cannot drift. Redaction stays
exactly-once per entity on every path (Redact/RedactHeader non-composability).
Root that is denied/missing/filtered → empty forest → the caller's existing
uniform 404. Documented scope caveat: multi-parent/cycle error policies
evaluate within the subtree.

**Files:** `internal/dataentry/gantt_handler.go` (+tests),
`.go-arch-lint.yml` (exclude gitignored `.ignored/`).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

Fast paths run ONLY under AllowAll verdicts, so no ACL row-gating is bypassed
(a scoped verdict cannot compose with the subtree predicate in one GraphQuery
— fallback, never approximation). `GetEntity` raw read for the root is gated
by the root type's verdict before any use. Field redaction unchanged
(RedactHeader carries the same verdicts). Gate→redact-once→fold→cap order
preserved on both paths.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

Fast-path == full-build equivalence (json-equal); deep-descendant fold
through the SQL closure; where-excluded root 404; existing uniform-404,
ACL-differential, truncation and policy tests all green on the new paths.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

Risk: fast-path drift from full-path semantics → mitigated structurally
(shared `finishGanttForest`, shared `addGanttNodes`) and by the byte-equality
test. Residual O(R) relation stream on drill noted in the report as the next
candidate (needs a from-set relation query the store lacks).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist~~ (N/A: no user-facing behaviour change — same wire
      contract, same semantics, faster)

## Design Review

- [x] ~~Run `/design-review`~~ (N/A-with-reason: the design was reviewed in
      the TKT-MW28U5 code review — RR-FJWAZS prescribed exactly this fix and
      its constraints; the report quantified it. A second full review of a
      reviewer-prescribed change would re-litigate settled findings.)
- [x] All critical/significant findings addressed in plan
