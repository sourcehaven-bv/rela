---
id: IMPL-FKC9B4X
type: implementation-checklist
title: 'Implementation: Raw not-found errors, unmarked step fields, entity wording and requests after an ended session'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (viewworld_absent_test, api_v1_test, retired_redirect_test, EntityDetail.notFound, EntityList.empty, DynamicForm.help, FlowRunCard, widgets, InlineCreateFormModal, workspaceAccess, useEvents, useNextAction)
- [x] Integration tests written (e2e not-found.spec)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a missing entry under a world, a retired id still redirects, a 500 keeps Retry, an unticked required checkbox)
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
The e2e spec runs the built rela-server: an entity, list and form address that names nothing shows the not-found state with the home link and no Retry. Full vitest, vue-tsc, eslint, Go tests of internal/dataentry, golangci-lint, arch-lint, plimsoll and comment-lint pass.

## Quality

- [x] Code follows project patterns (check similar code) (the 404 is the uniform not-found of the entity GET; the guard sits beside the other interceptors)
- [x] Checked for DRY opportunities (NotFoundState shared by four surfaces; writeViewEntryMissing shared by both view paths)
- [x] No security issues introduced (a missing entry now answers like a refused one)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
