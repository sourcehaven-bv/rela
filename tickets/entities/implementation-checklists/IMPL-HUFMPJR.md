---
id: IMPL-HUFMPJR
type: implementation-checklist
title: 'Implementation: Suspended, archived and ended-session states'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (workspaceAccess.test.ts, WorkspaceAccessScreen.test.ts, ReadOnlyBanner.test.ts, cluster_test.go, host_test.go, provision_test.go, staticshell_test.go)
- [x] Integration tests written (test full flow, not just units) (refusals go through the real API client and its interceptor; the host tests serve real requests)
- [x] Happy path implemented
- [x] Edge cases from planning handled (first refusal wins; login_url limited to local paths; probe cooldown; no identity; provisioning and erased stay unknown_tenant)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope (literal sentences pin user-facing text on purpose)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
Full vitest (472 files), vue-tsc, eslint, and go test for internal/tenant, internal/tenanthost and internal/dataentry pass. The states were reproduced in saas-dev before the fix; the fixed screens are covered by component tests, not re-checked in saas-dev.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors workspaceReadOnly.ts and tenantProvisioning.ts: a module ref set by the API client interceptor)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (login_url is limited to a local path on the same site; the gates only hide UI, the server still refuses)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
