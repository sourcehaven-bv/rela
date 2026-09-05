---
id: REV-EB11BO
type: review-checklist
title: 'Review: Create forms need a "Create & add another" button for repeated entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

| Check | Result |
|-------|--------|
| Go tests (`dataentryconfig`, `dataentry`) | PASS |
| `golangci-lint run` | 0 issues |
| `just comment-lint` (gate) | no unresolvable doc links across 13090 comments |
| `just coverage-check` | PASS — package floor and total (79.1%) both satisfied |
| `just arch-lint` | OK — no warnings |
| Frontend unit (vitest) | 2357 passed / 143 files |
| `vue-tsc -b` | clean |
| `eslint src/` | 0 errors (75 pre-existing warnings, none introduced) |
| e2e (playwright) | 285 passed, 0 failed — **two consecutive full runs** |

The e2e suite was run twice end-to-end specifically because an earlier flake in
this diff turned out to be a real defect (see RR-2QT4XY), not test noise. One
run was not enough evidence.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

Two reviewers ran: `cranky-code-reviewer` (quality) and
`rela-security-reviewer` (security). **The security review found nothing
attributable to this diff** — verified across the reset carrying no
server-redacted state, stale affordance verdicts being cleared and re-derived,
carried relation ids not exceeding what the same principal could post on a
single create, `return_to` never being re-read by the reset, the config key
failing closed at load, the Toast change reducing rather than widening the
click-sink surface, and the N-creates loop being equivalent to N page loads.

It did note two **pre-existing backend issues** (ungated relation-target reads,
and a create-path serializer passing `visibleNeighbors == nil`) that are
reachable identically with or without this diff. They are deliberately NOT
folded in here and are not filed — raising them is a separate call for Jeroen.

**Review Responses:** RR-9EAUGR (critical, addressed), RR-2QT4XY (significant,
addressed), RR-OPQU0M (significant, addressed), RR-GV1RVL (minor, addressed),
RR-DGN0VH (nit, wont-fix with reason). Plus the 12 design-review responses
carried from planning, all addressed.

Three defects were found AFTER implementation, each fixed and pinned:

1. **RR-9EAUGR (critical)** — `pendingCardChanges` was not cleared by the reset.
   An incoming RelationPicker DOES render in create mode and latches its
   selection into that map rather than into `relations`, so record N's incoming
   relation was written to record N+1 with the user touching nothing. My own
   code comment asserting the map is always empty in create mode was simply
   wrong. This is exactly the silent wrong-data class the reset exists to
   prevent, and my original test could not see it because the picker stub only
   counted mounts.
2. **RR-2QT4XY (significant)** — my first Toast fix was scoped too coarsely
   (`pointer-events: auto` on `.toast`), so the toast's icon and message spans
   still swallowed clicks aimed at the button underneath. Narrowed to
   `.toast-dismiss`.
3. **RR-OPQU0M (significant)** — no behavioural coverage existed for
   `keep_on_add_another` on a *relation*, despite the docs promoting that as the
   primary use case. That gap is why #1 shipped.

One reported critical was **downgraded on evidence** rather than accepted:
RR-GV1RVL claimed a kept relation loses its `pickerTypes` and aborts the second
save. The mechanism is real but unreachable in production — the reset's
`saveGeneration` bump remounts RelationPicker, whose `onMounted` re-emits
`update:types` for exactly this case. Confirmed by removing the carry-over and
watching the e2e stay green. The code keeps the carry-over (it makes the reset
self-sufficient) with a comment that says plainly it is belt-and-braces.

Diff self-review: no unrelated changes. `Toast.vue` and the e2e
`submitButton` locator narrowing look incidental but are both **caused by** this
feature — the toast overlaps the actions only because the user now stays on the
form, and the locator was a substring match that the new button's label broke.
Build output under `internal/dataentry/static/` is gitignored and not committed.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** all PASS. Full per-AC table with evidence is in
IMPL-X3OFMR; summary here:

| AC | Status |
|----|--------|
| 1 renders on create only (not edit, not embedded) | PASS |
| 2 payload identical to primary Create | PASS |
| 3 does not navigate | PASS |
| 4 clean reset, query pre-fills re-applied, template re-applied | PASS |
| 4b marked field/relation carries over, unmarked does not | PASS |
| 4c config key round-trips on FormField and FormRelation | PASS |
| 4d incoming picker cannot leak its selection | PASS |
| 5 second create from the same form instance | PASS |
| 6 toast names the created id | PASS |
| 7 not left dirty, incl. entry-locked field | PASS |
| 8 failure does not reset or claim success | PASS |
| 9 wizard returns to step 1 | PASS |

**Assertions are mutation-verified, not merely green.** Each was checked to
actually FAIL when its fix is removed: the kept-value re-apply, the
`createdEntityId` release, the `saveGeneration` bump, the `pendingCardChanges`
clear, the `dirty` re-baseline ordering, `wizard.goTo(0)`, and the Toast
`pointer-events` rule. That process caught one test of mine passing vacuously
and one over-specified test that failed against correct code.
