---
id: IMPL-121ACS5
type: implementation-checklist
title: 'Implementation: Relation trigger writes skip relation rules and strict validation'
started: "2026-11-27"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: not run by hand; integration tests drive the real manager and store)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by tests, not by hand)

**Verification Evidence:**
relation_trigger_checks_test.go: rule refusal, strict-mode refusal, advisory save, guard denial (403 + denied-write row), decimals as text, edge-dependent rule on create, relation_removed refusal; each runs via trigger and via PatchEntity on memstore and sqlite; the trigger cases fail on origin/develop. sqlite checks the relation write rolls back.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
