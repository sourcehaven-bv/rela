---
id: IMPL-S911A1
type: implementation-checklist
title: 'Implementation: Fuzz sweep: install Wails Linux deps for the build gate'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: workflow step only)
- [x] ~~Integration tests written~~ (N/A: workflow step only)
- [x] Happy path implemented (install step before the fuzz run)
- [x] ~~Edge cases handled~~ (N/A)
- [x] Error handling in place (apt failure fails the step)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: no planning checklist)
- [x] ~~Edge cases manually verified~~ (N/A)

**Verification Evidence:**

- Fuzz Sweep dispatched on `ci/fuzz-sweep-gtk` (run 39215430631): build gate passed, "Fuzz sweep OK: 55 targets, 25s each."

## Quality

- [x] Code follows project patterns (same install step as `ci.yml`)
- [x] No security issues introduced
- [x] No silent failures
- [x] No debug code left behind
