---
id: IMPL-RZE6Q7
type: implementation-checklist
title: 'Implementation: seqtrace: sequence diagrams of traced request flows'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed; the runtime deliberately drops trace-write errors so tracing never breaks the traced program)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests use small inline traces and a demo program; no domain fixtures apply)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

`just seqtrace-demo` on the postgres build: ten scenarios returned 200, 200,
404 (hidden risk), 200, 200, 200, 200, 403 (reader update), 201, 204. The
salary field was absent with `_redacted:["salary"]`. All ten diagrams parse in
Mermaid 11; index and per-diagram pages link correctly. `git status` after the
run shows only the intended files. Server and postgres were stopped by the
script's trap.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (CLI errors are returned and printed; runtime write failures are dropped on purpose, see above)
- [x] No debug code left behind
