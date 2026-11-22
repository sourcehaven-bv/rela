---
id: IMPL-ZOTHI3L
type: implementation-checklist
title: 'Implementation: Flow failure cause, notices and page follow-ups, and a flow runs panel gate'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (job_test, flows_handler_test, chrome_test, acl_audit_permissions_test, flowView, nextActionTarget, NextActionCard, Sidebar.groupAdd, useNextAction, EntityDetail.actions, InlinePropertyValue, FlowRunsPanel)
- [x] Integration tests written (e2e mobile-layout: menu bar title and column positions)
- [x] Happy path implemented
- [x] Edge cases from planning handled (exhausted retries, a fork branch failing, an unknown space, no acl.yaml)
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
A local server built from the branch on a copy of the saas-dev pool config: at 375px the entity page shows "Note" in the menu bar and list columns line up; the columns e2e fails without the CSS change. Full vitest, vue-tsc, eslint, Go tests of the changed packages, golangci-lint, arch-lint, plimsoll and comment-lint pass.

## Quality

- [x] Code follows project patterns (check similar code) (entity_panels uses the ChromeToggle shape of header_buttons)
- [x] Checked for DRY opportunities (storeJobStep shared by storeJob and parkFailed)
- [x] No security issues introduced (the panel gate only withholds data)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
