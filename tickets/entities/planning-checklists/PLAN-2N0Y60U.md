---
id: PLAN-2N0Y60U
type: planning-checklist
title: 'Planning: Budget tests count rows read, not only store calls'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**
In: rows counting in `storetest.Counting`, a scale helper, converting the
existing `storetest.Counting` budget tests to assert rows, and a test that a
single full-scan call fails the helper. Out: fixing any scan the conversion
finds (TKT-FED6FMM), and postgres `QueryStats` rows (it reports none).

**Acceptance Criteria:**
1. `Counting` counts rows yielded, paged and found by point reads, not
   counts. Test: `TestCounting_Rows`.
2. A helper asserts the rows read are the same at 100 and 1,000 entities.
   Test: `TestAssertScaleFree_PagedReadPasses`.
3. One full-scan call fails the helper on rows only. Test:
   `TestAssertScaleFree_FullScanFails`.
4. Budget tests assert rows as well as calls; a test that finds a real scan
   stays calls-only with `TODO(TKT-FED6FMM)`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small test-harness change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: extends the in-repo decorator)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo harness)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
`gantt_handler_test.go` had its own `ganttRowCounter`; it is replaced by the
shared counter. `store.QueryStats` has statements and duration, no rows.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
Charge rows in the decorator as the caller consumes an iterator. Compare
calls at 10 vs 50 served rows (N+1) and rows over a store padded to 100 vs
1,000 entities while the served output stays fixed (scans). Comparing rows
at 10 vs 50 served was rejected: output rows grow with n by design.

**Files to modify:**
`internal/store/storetest/counting.go`, new `scale.go` and `scale_test.go`,
the budget tests that use `storetest.Counting`, and
`.claude/rules/collection-reads.md`.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: test-only code)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: test-only code)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: test-only code)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: test-only code)

**Input Sources & Validation:** N/A, test code.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria.

**Edge Cases:** An iterator stopped early is charged only for what was
read; a missed point read charges no row; reads in a `Tx` view count on the
same counter.

**Negative Tests:** `TestAssertScaleFree_FullScanFails`.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: test-only code)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Slower tests from 1,000-entity fixtures; mitigated by padding
with header-only rows on memstore.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** `.claude/rules/collection-reads.md` only.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: test harness only, approach set by the ticket)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
