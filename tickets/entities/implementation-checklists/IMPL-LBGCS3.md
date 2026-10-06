---
id: IMPL-LBGCS3
type: implementation-checklist
title: 'Implementation: Text search misses faced entities after a search index rebuild'
started: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

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
- File project with a faced type and a world: before the fix, `/_search?q=`
  found no faced entity seeded on disk; after an API write it did. With the
  fix, a server started on the index the old build wrote rebuilt it, and every
  faced entity was found.
- Postgres with an ACL (type grant plus world grant), two roles: search and the
  `type:` listing found faced entities before and after the fix.
- e2e `faces-mention.spec.ts`: two file-backend tests failed before the fix
  (No matches); all four pass after it, including the postgres one.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
