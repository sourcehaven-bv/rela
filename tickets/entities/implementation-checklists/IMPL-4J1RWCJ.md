---
id: IMPL-4J1RWCJ
type: implementation-checklist
title: 'Implementation: Declarative peer pull: peers.yaml apply side'
started: "2026-11-08"
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
- `TestPeerPull_EndToEnd` (internal/dataentry) runs a real source router with a peer key: first pull creates 2 rows and their `blocks` relation, the ACL-hidden feature is not pulled, a second pull is a no-op, an upstream edit is applied and a field added locally through PatchEntity survives it.
- internal/peersync run tests: gone guard refuses a run over the threshold and applies nothing; 401/403 and an empty listing fail the run instead of marking rows gone; remote ids never become local ids; oversize bodies and invalid values are rejected; dry run writes nothing.
- internal/scheduler peers tests: a scheduled pull runs as `system:peer:<name>`, sends the bearer from secrets, keeps the secret out of errors, and shares the run lock with `rela peer pull`.
- `rela peer pull` CLI tests cover text and JSON reports, `--dry-run` and `--allow-gone`.
- Gates: golangci-lint 0 issues on changed packages; arch-lint, plimsoll, comment-lint and archguard pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
