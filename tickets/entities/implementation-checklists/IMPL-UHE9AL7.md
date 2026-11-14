---
id: IMPL-UHE9AL7
type: implementation-checklist
title: 'Implementation: Mac conventions in rela-desktop: reopen after closing the last window, Find, Print, Show in Finder, Help'
started: "2026-11-14"
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the window, Dock and print paths need the Wails runtime and a Mac session; they are checked by hand per the acceptance criteria)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the tests build small structs and DOM fragments inline)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values in the assertions)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [ ] Feature manually tested end-to-end
- [ ] Each acceptance criterion verified with test scenario from planning
- [ ] Edge cases manually verified

**Verification Evidence:**

Automated: `go test -race ./cmd/rela-desktop/...`, golangci-lint, plimsoll,
comment-lint, vitest for `useShellCommands`, vue-tsc and the SPA build pass.

Manual acceptance tests are pending; Jeroen runs them in a built
rela-desktop.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
