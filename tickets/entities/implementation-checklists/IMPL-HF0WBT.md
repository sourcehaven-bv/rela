---
id: IMPL-HF0WBT
type: implementation-checklist
title: 'Implementation: propose/commit seam'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `useProposal` (12), `useChangePolicy`
  (22), `useFormWizard` hypothetical evaluation (5), `useAutoSave` merging (9)
- [x] Integration tests written (test full flow, not just units) —
  `DynamicForm.propose.test.ts` mounts the real component and drives a real
  widget edit through to an asserted store write. This layer did not exist
  before and is where every BUG-FB0LN8 bug lived.
- [x] Happy path implemented
- [x] Edge cases from planning handled — self-hiding field, entity reload
  mid-dialog, unmount mid-dialog, empty field, redacted field, multi-field hide
- [x] Error handling in place — batch failures attribute to every property in
  the merged patch; superseded proposals return a named status rather than
  failing silently

## Test Quality

- [x] Using fixture builders or factories for test data — `mountEdit`,
  `formConfig`, `setup` helpers
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [ ] ~~Feature manually tested end-to-end~~ **NOT DONE — see below**
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — via mutation testing rather than by hand

**Verification Evidence:**

Verified by **mutation testing**, not by clicking: for each mechanism, the
implementation was deliberately broken and the suite re-run.

| Mutation | Tests that failed |
| --- | --- |
| Restore the destructive default (the original bug) | 3 |
| Defeat hypothetical evaluation | 3 |
| Remove pre-change retention | 3 |
| Invert retain/apply order | 1 |
| Revert to one-property-per-patch | 4 |
| Drop the all-suppressed guard | 3 |
| Remove the widget snap-back | 1 |
| Remove the navigation fence | 1 |
| Remove the unmount generation bump | 1 |

Two data-loss paths were found by adversarial review *after* the tests were
green, and both were reproduced before being fixed: a redacted `confirm` field
cleared with **0 dialogs shown**, and **1 PATCH** firing after unmount.

> **Manual browser testing has NOT been done.** That matters more here than
> usual: every one of the four previous `confirm` attempts passed its tests and
> then failed under a real click, and the fifth failure mode found in this work
> (the widget not snapping back on decline) was invisible to any assertion on
> `formData` — it lived purely in the DOM. The new tests assert DOM values and
> store calls specifically to close that gap, but they are not a substitute for
> someone using the form. **Recommend a manual pass before release.**

## Quality

- [x] Code follows project patterns (check similar code) — consumer-side
  interfaces (`ChangePolicyDeps`), composables over component growth
- [x] Checked for DRY opportunities — `wouldHide` / `proposedBindings` are
  shared pure helpers; `clearOnHide` deliberately does NOT re-derive the
  clear decision, which is what caused the redaction bug
- [x] No security issues introduced — `confirm` fails safe on a field the
  principal cannot read: no dialog, no clear
- [x] No silent failures — `superseded` is a named outcome, not a dropped promise
- [x] No debug code left behind
