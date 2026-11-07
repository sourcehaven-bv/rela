---
id: IMPL-D5V9LNC
type: implementation-checklist
title: 'Implementation: Scoped elevated updates for data-entry actions'
started: "2026-11-06"
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

- [x] Feature manually tested end-to-end (through the HTTP action endpoint in TestAction_ElevatedUpdates on a real manager and Declarative ACL)
- [x] Each acceptance criterion verified with test scenario from planning (scope, read gate, audit, fail closed, webhook refusal)
- [x] Edge cases manually verified (hidden row, out-of-scope property, content argument, illegal transition)

**Verification Evidence:**
- TestAction_ElevatedUpdates: in-scope update lands for a caller with no write grant; out-of-scope property, content and a hidden row are refused and nothing changes.
- TestAction_ElevatedUpdatesAudit: acl-bypass and update rows name bob with triggered_by action:submit.
- TestScopedUpdates_*: scope, read gate, audit, transitions with value-free errors.
- TestValidateActions_Elevation: load-time rules incl. alias, unique, computed and file properties.
- TestWebhook_RefusesElevatedAction and TestGrantElevatedUpdates: fail closed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
