---
id: IMPL-UHE9AL7
type: implementation-checklist
title: 'Implementation: Mac conventions in rela-desktop: reopen after closing the last window, Find, Print, Show in Finder, Help'
started: "2026-11-14"
completed: "2026-11-15"
status: done
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Automated: `go test -race ./cmd/rela-desktop/...`, golangci-lint, plimsoll,
comment-lint, vitest for `useShellCommands`, vue-tsc and the SPA build pass.

Manual, in a production build of rela-desktop on macOS:

- Close Window, then reopen the app (the Dock's reopen event): a main window
opens on the active project. Same after File > Open Recent and after opening the
project folder from Finder with no window.
- Edit > Find focuses the list's search box on All Tickets; on the dashboard
it opens /search. File > Print, Show in Finder and Help > Rela Help are present.
- Print of All Tickets: no sidebar or header buttons, two pages, cells keep
their ellipsis (checked with a Playwright print as well).
- Quit and Close Window both save the frame; relaunch restores it exactly
(1100x700 at 300,200 stays the same across cycles).
- Show in Finder reveals the project; it is disabled on the Clone from Git
screen.

Manual testing found three bugs, now fixed: RR-F1NDL57, RR-PR1NTC3, RR-S1ZEDR1.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
