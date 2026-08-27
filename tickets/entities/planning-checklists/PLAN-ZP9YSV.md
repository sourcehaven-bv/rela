---
id: PLAN-ZP9YSV
type: planning-checklist
title: 'Planning: storeutil.TopValues: hoist the triplicated property-value ranking (one copy had already drifted)'
status: done
---

<!-- @managed: claude-workflow v1 -->

Small mechanical extraction; sections are answered proportionally rather than
padded.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** IN — hoist the shared ranking tail of `PropertyValues` into
`storeutil.TopValues`; three call sites delegate; unit tests for the conventions
the copies disagreed on. OUT — the counting half (genuinely per-backend: prop
cache / entity scan / SQL) and any semantic change.

**Acceptance Criteria:** see the ticket. Each maps to a subtest in
`storeutil/topvalues_test.go`.

## Research

- [x] Checked codebase for similar patterns or reusable code
- [x] Reviewed relevant rela concepts for prior art
- [x] ~~/research~~ (N/A: xs mechanical extraction)
- [x] ~~External libraries~~ (N/A: twenty lines of stdlib sort)
- [x] ~~Reference implementations~~ (N/A)

**Existing Solutions:** `internal/store/storeutil` is the established home for
exactly this — its package doc says it exists "to avoid duplicating validation,
filtering, and sorted-slice maintenance logic". `SortedInsert` / `SortedRemove`
/ `PaginateSortedKeys` are the precedent.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** one exported function taking the already-computed counts
map. Deliberately NOT a method or an interface — the input is a plain map and
the output a plain slice, so there is nothing to abstract.

Alternative rejected: hoisting the *whole* `PropertyValues`. The counting half
differs fundamentally per backend, so a shared version would need a callback per
backend and be longer than the duplication it removes.

**Files to modify:** `storeutil/topvalues.go` (new),
`storeutil/topvalues_test.go` (new), `fsstore/entity.go`,
`memstore/memstore.go`, `pgstore/entity.go`.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

No new input reaches the process; the function is pure over an already-validated
map. One property worth stating: the result is returned to API callers, so it
must be non-nil when empty — `null` vs `[]` is a wire-format difference, pinned
by a test.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** ranking by count; alphabetical tiebreak; `limit` truncates;
`limit == 0` and `limit < 0` mean all; limit exceeding the set; empty map; nil
map. Integration coverage is the three existing conformance suites, which
exercise the real call sites unchanged.

**Edge Cases:** the allocation drift itself (asserted via `cap()`); determinism
across 50 runs, since Go randomizes map iteration and without the tiebreak two
backends could disagree; nil vs empty map.

**Negative Tests:** `limit < 0` must not be read as a cap (would return
nothing); nil counts must not return a nil slice.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (xs)

**Risks:**

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Silent behaviour change at a call site | Low | Three conformance suites run the real paths unchanged, pgstore against a live DB |
| Adopting fs/mem's buggy allocation as the shared version | Medium | Took pgstore's correct form and pinned it with a `cap()` assertion |
| Over-extraction (hoisting the counting half too) | Low | Explicitly out of scope; `sort` dropping out of all three files is the signal the cut was at the right seam |

**Effort:** xs.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

- [x] N/A — internal refactor, no behaviour change. The rationale (why
`limit <= 0` means all, why ties break alphabetically) lives in the function's
doc comment, where the next implementor will read it.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** none — this is an xs extraction of an existing,
tested code path with no interface or behaviour change. The design question that
mattered (extract the ranking only, not the counting) is recorded under
Approach. Full review was applied to the two substantive tickets in this batch
(TKT-415WA7, TKT-8TJ2WN).
