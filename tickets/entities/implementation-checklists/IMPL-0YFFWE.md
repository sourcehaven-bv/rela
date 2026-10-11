---
id: IMPL-0YFFWE
type: implementation-checklist
title: 'Implementation: Gantt: horizontal scroll with Now and arrow navigation'
started: "2026-10-09"
completed: "2026-10-09"
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

- `e2e/tests/gantt-scroll.spec.ts` (Chromium, real layout): a 300-day plan
opens with today centred (within 15px); › scrolls 360px (30 days × 12px), ‹
back; the tree column stays at the chart's left edge while scrolled; the bar
name stays visible right of the tree column; Now returns to today; switching
Month → Week keeps the left-edge day (1200px → 4000px). Both pass.
- Screenshots taken at today and scrolled back two months: sticky axis, tree
column and label confirmed visually. The first screenshot showed the label lost
when the bar started off-screen; fixed with a sticky label track and `overflow:
clip` (hidden made each row its own sticky container).
- Unit: `ganttLayout.test.ts` (ticks every period, withToday both sides and
out of reach), `GanttView.test.ts` (timeline width and labels, open at today,
arrows, Now, zoom anchoring, Now disabled far from today). 32 pass.
- Edge cases: today far outside the plan disables Now and opens at the start;
short spans stretch to fill (pxPerDay max with viewport/days); stale fetch
responses dropped via a sequence number.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
