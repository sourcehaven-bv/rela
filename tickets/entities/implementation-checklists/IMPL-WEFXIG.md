---
id: IMPL-WEFXIG
type: implementation-checklist
title: 'Implementation: Document view flashes empty state and scrolls to top on any unrelated entity write'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the
defect is entirely client-side render behaviour; the full flow was verified
manually against a running server with a real SSE event - see below)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Five tests in `frontend/src/views/DocumentView.rerender.test.ts`: body stays
mounted during an in-flight re-render, identical HTML preserves DOM node
identity, changed HTML still lands, a failed re-render keeps the document, and a
document switch still blanks. Errors continue to surface via the toast and
script-error panel - the change is that they no longer also destroy the view.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`response()` builds render payloads and `deferred()` holds a render open so the
in-flight window is observable. Selectors are constants (`BODY`, `EMPTY_STATE`).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Two `rela-server` builds of the same project (`prototypes/data-entry/project`),
fixed on :8099 and unfixed on :8098, driven through Chrome. A `MutationObserver`
counted body unmounts and empty-state appearances; an independent `EventSource`
counted SSE frames so "no flash" could be distinguished from "no event arrived".

Scrolled to 900px, then edited an UNRELATED entity (a `label`, while viewing the
standalone `status_review` document) to fire one real `entity:changed` frame:

| | unfixed (:8098) | fixed (:8099) |
|---|---|---|
| SSE frames received | 1 | 1 |
| body unmounts | 2 | 0 |
| empty state shown | 1 | 0 |
| scroll (from 900) | 0 - reset to top | 900 - held |

Both received exactly one event, so the difference is the fix, not a missing
trigger. The unfixed run reproduces all three reported symptoms (flash, empty
state, scroll reset).

Test-level mutation check: reverting both edits in `DocumentView.vue` fails 3 of
the 5 new tests; restoring them passes 5/5. The tests cannot pass against the
defect.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`frontend/CLAUDE.md` requires preserving the `v-if="loading && !docContent"`
guard in DocumentView/DocumentsPanel. The guard is untouched; this change makes
it work as its own comment already described.

DRY: the two loaders are deliberately NOT extracted into a shared composable.
They differ in cold-load trigger (route props vs. selected tab), return_to
construction, and error surface. Extracting ~15 lines across those differences
would be the premature abstraction CLAUDE.md warns about; the duplication is
noted in the bug body so both stay in step.

Full suite: 2658 tests / 164 files pass. `npm run typecheck` clean, `npm run
lint` 0 errors. Both touched files were already prettier-unformatted before this
change (verified by stashing) - not reformatted here to keep the diff
reviewable.
