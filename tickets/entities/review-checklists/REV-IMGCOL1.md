---
id: REV-IMGCOL1
type: review-checklist
title: 'Review: Image list column: thumbnail width and leading position'
started: "2026-11-23"
status: done
---

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (only `vitest run src/components/lists` was run, 231 passed, plus `vue-tsc --noEmit`; the Go suite was not run because no Go code changed; CI runs the full suite)
- [x] ~~Lint clean (`just lint`)~~ (N/A: no Go code changed; eslint on the changed frontend files reports 0 errors)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go code changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go code changed; the frontend has no coverage gate)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** the reviewer agent reported no critical and two significant findings (thumbnail could shrink; titles misaligned for rows without an image) plus minor ones. All were fixed: fixed-size aria-hidden slot per leading column, lock for an inaccessible image, one column list for table and name cell, de-duplicated keys, list-valued image properties not treated as images, doc comment placement. Not done: the reviewer's unverified guess that a "Logo" header is truncated at 64px (not rendered or checked), and extra tests for the no-URL fallback, two leading columns and grouped lists. No review-response entities were created.

## Acceptance Verification

- [x] ~~Each acceptance criterion tested (reference planning checklist)~~ (N/A: no planning checklist; width, leading position and empty-slot cases have vitest cases)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: no implementation checklist; evidence is the vitest cases in tableColumns.test.ts and EntityList.cells.test.ts)

**Acceptance Status:** width: PASS (unit test). Leading image column in the name cell: PASS (unit tests). Not checked in a browser.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: docs updated in the same change, no separate checklist)
- [x] User-facing documentation updated
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist)

**Docs Checklist:** N/A

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] ~~Ready for another developer to use~~ (N/A: needs the human PR review)

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR is opened by this session after the ticket is done)
