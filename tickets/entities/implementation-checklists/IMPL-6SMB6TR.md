---
id: IMPL-6SMB6TR
type: implementation-checklist
title: 'Implementation: Give the whole-set scopedHeaders callers their own read function'
started: "2026-11-23"
completed: "2026-11-24"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal read-path refactor with no UI change; covered by golden parity and budget tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** dataentry and archguard tests pass on default,
memorybackend and sqlite builds. New tests:
TestScopedHeaders_UnboundedReadIsRefused,
TestScopedHeaders_WindowReadsOneWindow,
TestScanHeaders_RefusesAQueryForAnotherType,
TestScanHeaders_TakesItsQueryFromScopedQuery, TestScanCallProblem,
TestLLMAction_UnconditionedCandidatesScaleFree,
TestLLMAction_UnconditionedCandidatesFillExactly,
TestDynamicCollections_ReadIsScaleFree,
TestUnpagedReadAllowlist_RequestPathNamesItsTicket. Golden parity cost lines
updated.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
