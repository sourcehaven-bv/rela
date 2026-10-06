---
id: IMPL-Q8V775
type: implementation-checklist
title: 'Implementation: Autosave conflict resolution: per-field version preconditions, three-way merge on 412, bounded auto-retry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (fieldversions_test.go: token, scope validation; autoSaveMerge.test.ts: text/property/relation merge)
- [x] Integration tests written (TestV1UpdateEntity_* through the real HTTP handler, incl. a lost CAS race and a real fsstore reformat; useAutoSave.conflicts.test.ts drives the composable end to end against a mocked store)
- [x] Happy path implemented
- [x] Edge cases from planning handled (absent-field token, redacted field has no token and is not an oracle, precondition on an unwritten field → 400, 412 naming no field → resend, bounded 3 attempts, base never runs ahead of a dirty field)
- [x] Error handling in place (a conflict sets the field/content error, status `error`, and calls onError with status 412)

## Test Quality

- [x] Using fixture builders or factories for test data (existing dataentry test app; `harness()` and `versions()` in the frontend tests)
- [x] No hardcoded values in assertions when object is in scope (tokens read back from the GET before being asserted)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

rela-server built from this branch, run on a copy of
`prototypes/data-entry/project` (fsstore).

API (curl):
- GET /api/v1/tickets/TKT-001 returns `_versions` with a token per declared property, plus content and relations tokens.
- PATCH status with a matching precondition → 200 and a new status token.
- PATCH with the old status token → 412 `application/problem+json` with `conflicts.properties.status` {expected, actual} and full `versions`; file unchanged.
- PATCH status with a precondition on `title` → 400 `invalid_precondition`, detail `/preconditions/properties/title`.
- PATCH content with a stale content token → 412 naming `content`.
- PATCH a table body: the content token in the PATCH response equals the token of a later GET (fsstore reformat does not invalidate it).

Browser (entity detail page, autosave):
- Another client changed status and description. Editing priority in the stale page saved cleanly; the server kept the other client's status and description, and the form picked them up from the response.
- Another client set priority=critical; the stale page set priority=medium. The inline field message "Someone else changed this field..." appeared, the server kept `critical`, and the form kept `medium`.
- Editing priority again (low) overwrote deliberately: the server stored `low` and the inline message cleared.
- The body merge is covered by unit tests only, not by a browser check.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (shared `writeFieldConflict`, `currentVersions`; merge rules in one module)
- [x] No security issues introduced (tokens come from the wire projection, so redacted values yield no token and a precondition on them is refused like a write)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
