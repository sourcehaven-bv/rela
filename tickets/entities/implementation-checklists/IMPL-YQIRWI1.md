---
id: IMPL-YQIRWI1
type: implementation-checklist
title: 'Implementation: seqtrace: record the same-package functions between drawn calls'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: `Build` is a pure function; the demo run exercised it end to end)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error path added)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the test reuses the package's existing sample call tree)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: the expected chains are the point of the test)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: the expected chains are the point of the test)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

1. to 3. `go test ./tools/seqtrace/...` passes, including both `TestStepIn` cases.
2. The existing fold and diff tests pass unchanged.
3. End to end: the route suite of `just seqtrace-demo` wrote a `steps.json`
with `in` on its steps. An archplan build that reads it reported 2
repeated-orchestration findings where it reported 9 before.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no error path added)
- [x] No debug code left behind
