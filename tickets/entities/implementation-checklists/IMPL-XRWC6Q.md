---
id: IMPL-XRWC6Q
type: implementation-checklist
title: 'Implementation: pgstore range filters use database collation; empty ~= fails 42P18'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestGraphDifferential on pgstore)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: store-level SQL fix; the differential test runs the real database)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty value; mixed case and punctuation)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths)

## Test Quality

- [x] Using fixture builders or factories for test data (seeded generator)
- [x] No hardcoded values in assertions when object is in scope (compares with graphquerynaive)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no user-facing surface; covered by the differential run)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** The pgstore differential run went from 19 divergences
in 400 queries to 0 against a local PostgreSQL with an `en_US.UTF-8` database.
The full pgstore suite, including the EXPLAIN tests, passes after the `COLLATE
"C"` change.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (values stay bound; the collation name is a constant)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
