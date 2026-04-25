---
id: IMPL-F4EH2
type: implementation-checklist
title: 'Implementation: Add Edit button to data-entry document view'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**What was implemented:**

Backend (`internal/dataentryconfig/`):

- Added `DocumentEdit{Form, Label}` struct and `Edit *DocumentEdit` field
on `DocumentConfig` (`config.go`).
- Extended `validateDocuments` in `validate.go` with three checks: empty
form, empty label, and unknown form reference (the last mirrors the existing
`list.edit_form` / `kanban.edit_form` validation pattern).
- Added four new table-driven cases in `validate_test.go` covering happy
path + each negative.

Frontend:

- `frontend/src/types/config.ts`: mirrored `DocumentEdit` interface and
added `edit?: DocumentEdit` to `DocumentConfig`.
- `frontend/src/views/DocumentView.vue`: added `editConfig` computed,
`editEntity()` handler, and the gated button. Note explicitly documents the
deliberate divergence from `EntityDetail.vue` (which has no `return_to` because
it's reached via SPA history; DocumentView is deep-linkable, so `router.back()`
from a fresh tab leaves the SPA). Added `gap: 8px` to `.header-right` so the
Edit and Refresh buttons don't sit flush.

Tests (`/e2e/`):

- New `e2e/pages/document.page.ts` page object (`navigateToDocument`,
`editButton(label)`).
- New `e2e/tests/document-edit-button.spec.ts` with 4 tests:
AC1 (button visible when configured), AC2 (navigation + decoded `return_to`),
AC3 (form save returns to document URL), AC4 (button absent when `edit:` is
omitted).
- Two docs added to the fixture's `DATA_ENTRY_YAML`: `feature_summary`
with the `edit:` block; `feature_readonly` without (the negative case).

Docs:

- `docs/data-entry.md`: paragraph in the Documents section explaining
the `edit:` block, plus the YAML example extended to show it.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The Go validation tests are table-driven (existing pattern) and only specify the
property under test plus the minimum required fields. The e2e tests use a
`DocumentPage` page object and assert on the configured label / configured form
id, not literals beyond what's authored in the fixture.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The 4 e2e tests in `document-edit-button.spec.ts` all pass against a real
`rela-server` binary serving the SPA build, exercising:

1. AC1 — button rendered when `edit:` is configured (`feature_summary`).
2. AC2 — click navigates to `/form/feature/FEAT-001` and `return_to`
round-trips via `URLSearchParams.get` (not coupled to encoding).
3. AC3 — submitting the form (no-op PATCH on FEAT-001) lands back on
`/document/feature_summary/FEAT-001` with the body re-rendered.
4. AC4 — `feature_readonly` (no `edit:`) shows no Edit button while
Back/Refresh are still present.

The Go validation tests (`TestValidateConfig_Documents`) cover happy path +
three negatives (unknown form, empty form, empty label).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The validation logic mirrors the existing `list.edit_form` / `kanban.edit_form`
checks. The Vue handler reuses `buildReturnTo`, which already enforces the
same-origin guard for `return_to`. No new shell / file / network surface
introduced.
