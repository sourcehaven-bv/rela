---
id: IMPL-35DM3WE
type: implementation-checklist
title: 'Implementation: Read-only record actions, ways out of a closed workspace, banner placement, people list and dark mail body'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (workspaceReadOnly, EntityDetail.header, WorkspaceAccessScreen, ReadOnlyBanner.placement, me tests; staticshell, account, mailrender compat tests)
- [x] Integration tests written (TestProvision_MePost drives GET and POST _me through the router with a verified JWT)
- [x] Happy path implemented
- [x] Edge cases from planning handled (invalid links and an over-long contact are dropped from the shell; a refused POST keeps the GET answer; a second POST creates no second person)
- [x] Error handling in place (a failed provision keeps the unmatched principal, as on the write path)

## Test Quality

- [x] Using fixture builders or factories for test data (provisionApp, the existing mount helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the shared saas-dev environment is in use by other work; covered by the router-level Go test and component tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (by the tests above)

**Verification Evidence:**
Focused Go tests for dataentry, dataentryconfig and mailrender pass; vitest over common, api, utils, entity, composables and forms (336 files) passes; vue-tsc passes.

## Quality

- [x] Code follows project patterns (check similar code) (shell meta like rela-app-name; provision through newProvisionSeam; account links through ValidAccountLink)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (links pass the account link rule; values are HTML-escaped into the shell; provisioning stays a POST under the same-origin check)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
