---
id: REV-QOMZWQ
type: review-checklist
title: 'Review: Milkdown editor renders GFM task lists as bullets with no checkbox'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go files changed; the frontend has no coverage enforcement)

2727 frontend unit tests pass across 168 files. `vue-tsc` clean. eslint 0 errors
(125 pre-existing warnings, down from 127). `just comment-lint` clean, though it
only scans `./internal ./cmd` so this diff is out of its scope. `just arch-lint`
OK. Prettier clean on every changed file. 27 e2e tests pass in Chromium
(checkboxes, forms, entity-refs).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-IZZROK (critical), RR-465KMS (critical), RR-H2LHT7
(significant), RR-1HWK9U (significant), RR-9R512E (significant), RR-N0WRN5
(minor), RR-SG15U4 (minor), RR-OK6E3Z (minor). All eight `addressed`.

The review found two genuine critical defects, both in the same blind spot:
every unit test drove the toggle with a synthetic `mousedown` and a collapsed
cursor, so neither keyboard activation nor a ranged selection was ever
exercised.

1. **Keyboard activation lost edits** (RR-IZZROK). Space fires `click` with no
`mousedown`, so the box ticked while the document did not, and the tick reverted
on the next redraw with nothing for the guard to catch.
2. **The command converted one item of a multi-item selection** (RR-465KMS),
with no signal the operation had been partial.

Fixing the first surfaced a further trap worth recording: the obvious fix
(`preventDefault` on the click) passed in jsdom and **failed in Chromium**,
because canceling a checkbox's click makes the browser restore the pre-click
checkedness after the handler returns. The final version does not cancel the
click; it lets the browser's flip stand and makes the document agree, with
`update()` authoritative on later redraws.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Task lists render as real checkboxes reflecting checked state — **PASS**
(unit + e2e + screenshot).
- The checked state is visually distinguishable — **PASS** (e2e asserts the two
seeded items differ).
- No bullet beside the checkbox — **PASS** (e2e asserts computed
`list-style-type: none`, which needs `:has()` and so a real browser).
- A checkbox can be toggled by mouse and keyboard, changing the document —
**PASS** (e2e asserts box and markdown together for both paths).
- A task list can be created and reverted from the toolbar — **PASS**,
including across a selection and from a bare paragraph.
- Markdown round-trips with no spurious emit — **PASS** across nested, ordered,
loose, multi-paragraph, blockquoted, emphasised and entity-ref shapes; `*` and
`[X]` spellings are correctly `churn-suppressed`.

Non-vacuity verified for the whole suite: 12 of 15 original unit tests failed
against the pre-fix editor, both e2e rendering tests fail with the node view
disabled, the keyboard e2e test fails against the old `mousedown` binding, and
each CSS rule was confirmed load-bearing by removing it.

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

Although this is a bug, the fix adds a visible toolbar control, so the manual
would have been wrong to leave alone. `docs-project/entities/guides/
GUIDE-data-entry.md` gains the task-list button in the toolbar list plus a short
section on creating and ticking items; `docs/data-entry.md` regenerated via
`just docs`. The undo claim in that text was verified before it was written, and
is now a test.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on the bug already being `done`, so this item cannot be satisfied before the checklist closes — see TKT-UFV01M and the note below)
