---
id: IMPL-Y6ZRZPV
type: implementation-checklist
title: 'Implementation: Lint: no comments in function bodies (Go and TS)'
started: "2026-11-24"
completed: "2026-11-24"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests use inline source snippets)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: assertions count findings, no objects in scope)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no object properties compared)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test ./tools/commentbody`, `go run
./tools/commentbody`, `npm run lint:comments` and vitest for the rule pass on
the real tree. Baselines regenerated after rebase; CI gates run the same
commands.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or patterns extracted to a helper / constant / type where it sharpens the contract (don't extract for its own sake; CLAUDE.md "three similar lines is better than a premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
