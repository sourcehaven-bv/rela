---
id: IMPL-4WPUSAJ
type: implementation-checklist
title: 'Implementation: Flows M1: run flows without channels'
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the CLI and ran it against a copy of `internal/flowcfg/testdata`:
`rela flow validate` reports the sample as valid; `rela flow show` lists
the flow with its version; `rela flow show onboarding` prints the Mermaid
diagram; `-o json` works for both; a broken target prints the keyed problem
and exits 1; `rela flow /tmp/nope.lua` still dispatches to the script
runner. `go test -race ./...` passes; `just arch-lint` is clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
