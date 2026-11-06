---
id: IMPL-O5WTDUI
type: implementation-checklist
title: 'Implementation: Validation rules enforced on write (enforce: write)'
started: "2026-11-06"
completed: "2026-11-06"
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

**Verification Evidence:** Built `rela` from the branch and ran it on a temp
project with a required `title`, an enum `status` and an enforced rule
`rechtsgrond-bij-beoordeling`:
- create with status=beoordeling and no rechtsgrond: refused, "validation rule ... is not satisfied"; nothing stored.
- create without title, advisory: stored with a warning. Same with RELA_VALIDATION=strict: refused, nothing stored.
- create with an unknown enum value under strict: refused.
- update to beoordeling without rechtsgrond: refused, file unchanged. Update with both set: accepted.
`go test ./internal/... ./cmd/...` passes, apart from the rela-desktop SPA test
that needs `just build-frontend`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
