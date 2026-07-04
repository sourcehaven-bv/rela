---
id: IMPL-C0LALM
type: implementation-checklist
title: 'Implementation: Reposition Properties auto-save indicator inline in the section heading, hidden when idle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (component-level: SectionEditForm heading row + indicator; AutoSaveIndicator visibility states)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (error state wins over idle-hidden, stays visible)

## Test Quality

- [x] Using existing fixture builders (`makeFields`, `mountForm`) for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: state/class assertions)
- [x] ~~Property comparisons use original object~~ (N/A: DOM/class assertions)

## Manual Verification

- [x] Feature manually tested end-to-end (real app: rela-server on tickets project, ticket detail page)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran `rela-server` (built from this checkout, embedding the freshly built SPA)
against the in-tree `tickets/` project; drove the ticket detail page
`/entity/ticket/TKT-VZB9O` (writable properties section → SectionEditForm) via a
headless browser.

- **AC1 (idle hidden):** at idle the indicator is `data-visible="false"`, class
`autosave-indicator autosave-saved autosave-hidden`. Idle screenshot shows the
"Properties" heading with a clean full-width underline and NO floating checkmark
in the corner (the reported bug). ✅
- **AC2 (placement while saving):** measured geometry — indicator right edge
= header row right edge = 1376px; indicator top = header top = 225px. Inline,
right-aligned, on the same line as "Properties". ✅
- **AC2/AC3 (lifecycle):** polled status on a real edit:
`idle(hidden) → saving(visible, spinner) → saved(visible, held >1s) →
idle(hidden)`. The saved → idle flip fades out over the 0.3s opacity transition.
Timing comes from `useAutoSave`'s existing `SAVED_INDICATOR_MS=1200` /
`MIN_SAVING_VISIBLE_MS=600` (unchanged). ✅
- **AC4 (error persists):** unit test — error state stays visible, not
`autosave-hidden`, even when status is idle but an error is present. ✅
- **AC5 (no regression):** cards/list `#indicator` slot path unchanged; full
suite green.

Automated: `npm run typecheck` (0 errors), `npx eslint src` (0 errors), `npm run
test:run` (71 files, 1145 tests pass — includes 6 new AutoSaveIndicator tests +
3 new SectionEditForm heading-row tests).

## Quality

- [x] Code follows project patterns (reuses the established `#indicator` slot pattern from cards/list; heading owned by form via `heading` prop rather than a template ref or Teleport — #997-safe)
- [x] Checked for DRY opportunities — heading-row CSS mirrors EntityDetail's `.section-heading` intentionally (scoped styles don't cross components); no premature abstraction
- [x] No security issues introduced (presentational-only)
- [x] No silent failures
- [x] No debug code left behind
