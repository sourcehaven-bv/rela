---
id: IMPL-ULSHN7
type: implementation-checklist
title: 'Implementation: `_self` on a non-bare face 404s under a configured default_world'
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

**Verification Evidence:** `rela-docs build
atlas-world/verify/atlas-worlds-verify.md` passes all 19 claims including the
issue-9 acceptance test (was failing by design before). UI checklist walked in
Chrome on the verify project as `ciso@example.com` with `default_world:
actueel`: POLICY-002 (concept) shows Edit/Vaststellen/Delete and no note;
POLICY-001 (adopted) shows the read-only note, no Edit, "View Concept";
`/list/taken` shows delete affordances and no world note;
`/form/edit_beleid/POLICY-002` opens editable;
`/form/edit_beleid/POLICY-001@vastgesteld` explains the face; Vaststellen on
POLICY-002 toasts "Vaststellen: Vastgesteld created" and lands on
`POLICY-002@vastgesteld`; "View Concept" lands on `POLICY-002@concept` with
Edit. Go suite and 2351 SPA tests green; golangci-lint, arch-lint, comment-lint,
plimsoll clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
