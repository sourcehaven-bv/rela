---
id: IMPL-KUG1H6
type: implementation-checklist
title: 'Implementation: Pin extractions/setup-just to a commit SHA'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: workflow config change, no code)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the Archplan CI job on the PR exercises the pinned action)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: verified by the Archplan job on the PR, not locally)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `grep -rn setup-just .github` shows exactly two uses,
both `@f8a3cce218d9f83db3a2ecd90e41ac3de6cdfd9b # v3.1.0`. `gh api
repos/extractions/setup-just/git/ref/tags/v3.1.0` returns object type `commit`
with that SHA (lightweight tag; commit message "This is 3.1.0"). actionlint
reports no issue on the changed lines (only pre-existing runner-label and
shellcheck notes elsewhere). AC3 is checked by CI on the PR.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: two one-line pins)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no code)
- [x] No debug code left behind
