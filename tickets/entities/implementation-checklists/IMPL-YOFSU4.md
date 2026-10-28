---
id: IMPL-YOFSU4
type: implementation-checklist
title: 'Implementation: Data-entry answers 500 instead of 401 when a handler finds no bound identity'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestAuthorizeWrite_NoRequestScopeKeepsCause, TestNoIdentity_SharedMappers)
- [x] Integration tests written (test full flow, not just units) (TestNoIdentity_Status drives the v1 routes with the Declarative wired into read gate, affordances and entitymanager)
- [x] Happy path implemented
- [x] Edge cases from planning handled (mismatch and foreign identity stay 500; uniform 404 kept when nothing is bound; rule deny joined with ErrNoIdentity stays 403)
- [x] Error handling in place (errors surfaced, not swallowed) (401 and 500 paths log the cause; bodies carry no error text)

## Test Quality

- [x] Using fixture builders or factories for test data (newNoIdentityApp over appbuildtest)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (probe over registerAPIV1Routes in four ctx modes before and after the fix)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Before: POST 403 "no request scope", PATCH 500
acl_query_failed, DELETE 500 delete_failed. After: 401 unauthenticated for all
three; mismatch 500 on all three; nothing bound keeps 404 on GET/PATCH/DELETE
for existing and missing ids. Reverting writeNoIdentity makes
TestNoIdentity_Status fail on the POST, PATCH and DELETE cases.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (writeNoIdentity and writeIdentityFaultDeny
are the single mapping points)
- [x] No security issues introduced (rela-security-reviewer: no critical or significant findings)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
