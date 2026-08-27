---
id: IMPL-VR6
type: implementation-checklist
title: Implementation
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

Verified end-to-end against a real project: the exact outage config now reports DENY with exit 1, naming rule_kind=role-grant; adding the grant flips to ALLOW via relation_grants with exit 0. Both outputs quoted verbatim in docs/acl-overview.md and re-run against the built binary to confirm they match. Exit codes checked for typo'd relation type, missing entity, and denied verb (all 1). JSON output shape matches the existing acl can convention.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
