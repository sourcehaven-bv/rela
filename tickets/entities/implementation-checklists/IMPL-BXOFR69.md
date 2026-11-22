---
id: IMPL-BXOFR69
type: implementation-checklist
title: 'Implementation: Settings and About gating, a read-only workspace without pile changes'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (settings_authz_test, me_handler_test, builtins_test, account_test, help_docfields_test, SidebarFooter.about, PileToggles, PilePanel, EntityDetail.world)
- [x] Integration tests written (e2e settings-gating)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no acl.yaml, a read-only workspace, a failed sidebar load, an invalid link after a live reload)
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
The e2e settings-gating spec runs a server built from the branch: an editor gets the shared cards, a role without settings:edit gets only its own preferences, builtins hide both footer buttons, and About shows the version and links. Full vitest, vue-tsc, eslint, Go tests of the changed packages, golangci-lint, arch-lint, plimsoll, comment-lint and markdown lint pass.

## Quality

- [x] Code follows project patterns (check similar code) (settings and about join the builtins rows of the Shortcuts button; can_edit_settings follows can_remote_sync)
- [x] Checked for DRY opportunities (permitsSettingsEdit shared by the save routes and /_me)
- [x] No security issues introduced (gates are for usability; every save still checks the permission)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
