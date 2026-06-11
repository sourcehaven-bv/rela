---
id: IMPL-BJMQ
type: implementation-checklist
title: 'Implementation: Migrate checkbox toggle to PATCH-based reactive flow; retire /api/toggle-checkbox'
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests are pure functions of input strings; no fixtures needed)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: tests assert on string-toggling output that's deliberately content-specific)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: tests compare string outputs, not domain objects)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Verified the marked v17 bullet acceptance set programmatically (`marked.parse` against `-`, `*`, `+`, `N.`, indented, no-trailing-space inputs); used to widen the toggler regex to match (RR-T8TV).
- 801/801 frontend unit tests pass — added 14 in `checkboxToggle.test.ts` covering all four bullet shapes, multi-digit ordered, indentation preservation, CRLF preservation, and the GFM-spec edge case `- [ ]nospace` (correctly rejected).
- 199/199 e2e tests pass — including the new `toggling a checkbox does not flicker the entity detail tree` test that installs a MutationObserver to deterministically catch any appearance of `.entity-detail > .loading-state` during the click.
- 52/52 Go test packages pass — confirmed via `go test ./...`. The deleted `TestToggleCheckbox` and `TestCheckboxStats` tables removed cleanly with no other consumers.
- arch-lint, typecheck, lint (0 errors) all clean.
- Net diff: −233 / +118 LOC (server side gets simpler; client side gets the toggle logic + reactive splice).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
