---
id: IMPL-PV6REC
type: implementation-checklist
title: 'Implementation: Gantt: drag bars to move and resize'
started: "2026-10-10"
completed: "2026-10-10"
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

- `e2e/tests/gantt-drag.spec.ts` (Chromium): hovering a plan bar shows the
handles; dragging the bar two days right writes start and end +2 (read back
through the API) and does not drill; dragging the end edge one day left changes
only the end; focusing the name, Tab to the start slider, two ArrowRight and
Enter move only the start by two days. Passes, as does gantt-scroll.
- Screenshot of the keyboard preview: dashed outline on the previewed window,
focus ring on the start handle.
- Unit: `useGanttDrag.test.ts` (29): shift and clamp, canDrag (update refused,
read-only, hidden, empty), face address, one GET per address after the dwell, no
GET when the pointer leaves or the node lacks dates, move/edge/datetime writes
with preconditions, threshold click vs drag, click suppression, zero delta,
Escape and cancel, 403/412 message and reload, refusal at commit, warnings,
keyboard preview and Enter, Escape and blur. `GanttView.test.ts`: no handles
when update is refused, three sliders in order and a keyboard write with one
refetch, a click on the window drills. Go: `TestGanttEmit_NamesTheFace`.
- Found during e2e: a short plan stretches the scale, so the spec measures a
day off the bar. Found in the view test: the handlers receive the drawn node
(preview applied), so the preview now records the window it started from.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
