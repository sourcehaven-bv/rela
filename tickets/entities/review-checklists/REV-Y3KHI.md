---
id: REV-Y3KHI
type: review-checklist
title: 'Review: Relation pickers should display name + id, not id alone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`npm run test:run` — 482/482 frontend tests pass; Go tests N/A — no backend changes)
- [x] Lint clean (`npm run lint` — 0 errors; new files emit 0 warnings)
- [x] ~~`just coverage-check`~~ (N/A: that target runs Go coverage. Frontend uses a per-file ratchet — new test file adds coverage for previously-untested component.)
- [x] Typecheck clean (`npm run typecheck` — vue-tsc clean)

## Code Review

- [x] Ran cranky-code-reviewer agent on the diff
- [x] All critical responses addressed (none flagged)
- [x] All significant responses addressed (1 fixed, 1 wont-fix with documented justification)
- [x] Self-reviewed the diff for unrelated changes (only RelationPicker.vue + new test file)

**Review Responses:**

| ID | Severity | Status | Notes |
|----|----------|--------|-------|
| RR-XIEIP | significant | addressed | Replaced `String()` coercion with `typeof === 'string'` guard; added test for object-typed title |
| RR-3ZX6Z | significant | wont-fix | Pattern matches established `EntityList.test.ts` convention; project-wide refactor is a separate concern |
| RR-0UQKL | minor | deferred | Out of scope per ticket; warrants future shared util ticket |
| RR-YP8UW | minor | deferred | Pre-existing CSS limitation; not introduced by this change |
| RR-9PA0Q | nit | addressed | Renamed describe block |

## Acceptance Verification

| AC | Status | Evidence |
|----|--------|----------|
| AC1 — chip with title shows `Title (ID)` | PASS | Unit test `selected chip shows "Title (ID)" when entity has a title` |
| AC2 — chip without title shows id alone | PASS | Three unit tests (missing, empty/whitespace, non-string) |
| AC3 — dropdown items consistent format | PASS | Unit test `dropdown items use the same "Title (ID)" / "ID" format` |
| AC4 — search by id still works | PRESERVED | Existing e2e at `e2e/pages/form.page.ts:96-98` searches by id substring; new rendering still contains id |
| AC5 — type pill preserved | PASS | Template inspection — type pill spans untouched in chip and dropdown |

## Documentation

- [x] N/A — internal UI tweak. No user-facing docs reference relation picker rendering.

## Final Checks

- [x] Commit message will explain the WHY (selected chip showed only title-or-id fallback, hard to identify entities)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred: user has not asked for a PR; will run on request)
- [x] ~~All CI checks pass~~ (deferred along with PR creation; local equivalents — test/lint/typecheck — are green)
- [x] ~~PR URL documented below~~ (deferred along with PR creation)
