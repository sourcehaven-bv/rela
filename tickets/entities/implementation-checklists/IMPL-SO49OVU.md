---
id: IMPL-SO49OVU
type: implementation-checklist
title: 'Implementation: Tenant pages keep fetching, fixed read-only text and jargon in labels'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (workspaceReadOnly, WorkspaceAccessScreen, ReadOnlyBanner, AccountMenu, staticshell, account config)
- [x] ~~Integration tests written~~ (N/A: rela's e2e suite runs a single-tenant server, which has no suspended state)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a status that does not answer, a 401 or 403 on the status, provisioning, the first refusal stays, Retry)
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
rela-server with the change behind a proxy that reports a suspended workspace: the page shows the suspended screen, the tab reads "Werkruimte opgeschort · rela", and together with BUG-JT865RH no API request other than the status is sent. Without the proxy the app starts as before. The shell carries `rela-read-only-text` when `account.read_only_text` is set. Full vitest, vue-tsc, eslint, Go tests of internal/dataentry and internal/dataentryconfig pass.

## Quality

- [x] Code follows project patterns (check similar code) (read_only_text follows suspended_text: config key, shell meta, bootMeta)
- [x] Checked for DRY opportunities (the status read is the one that already set the read-only flag)
- [x] No security issues introduced (the text is plain, escaped in the meta and capped at 500 characters)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
