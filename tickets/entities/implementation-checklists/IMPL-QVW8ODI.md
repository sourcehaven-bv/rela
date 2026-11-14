---
id: IMPL-QVW8ODI
type: implementation-checklist
title: 'Implementation: Push desktop notification conditions into the store'
started: "2026-11-14"
completed: "2026-11-14"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: the desktop app needs the OS notification center; the notifier loop is covered by cmd/rela-desktop tests against a memstore)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test -race ./internal/desktopnotify/
./internal/archguard/` and the notifier tests in `./cmd/rela-desktop/` pass. The
budget tests fail when the pushdown is disabled (rows 103 vs 1003) and when the
rule query's Limit is removed (rows 1100 vs 2000). golangci-lint, arch-lint,
comment-lint and plimsoll are clean on the touched packages.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
