---
id: IMPL-6FGQPC2
type: implementation-checklist
title: 'Implementation: RELA_AFFORDANCE_PROFILE=none/demo disables client field ceilings on REST'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestResolverFromProfile_Selection, 13 cases)
- [x] Integration tests written (test full flow, not just units) (TestNewServer_AffordanceProfileWithFieldRules runs newServer over a real fs project)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil declarative, no grants, ceiling only, unknown profile, empty profile)
- [x] Error handling in place (errors surfaced, not swallowed) (wraps ErrProfileOverridesFieldRules; startup exits)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: inline acl.yaml strings are the subject under test; uses newTestAppV1 / newConfigProject fixtures)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (error text built from tc.profile)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built rela-server and ran it on a scratch project in
.ignored/mproj whose acl.yaml redacts task.notes for app clients.
- `RELA_AFFORDANCE_PROFILE=none`: exit 1, `build affordance resolver: dataentry: acl.yaml declares field rules: RELA_AFFORDANCE_PROFILE=none would drop them on REST while MCP and CLI still apply them; unset RELA_AFFORDANCE_PROFILE`.
- `RELA_AFFORDANCE_PROFILE=demo`: same error with `=demo`.
- Profile unset with the ceiling: server starts.
- `RELA_AFFORDANCE_PROFILE=none` with an acl.yaml without field rules: server starts.

## Quality

- [x] Code follows project patterns (check similar code) (same error path as the DR-M4 compile error)
- [x] Checked for DRY opportunities (the grants check is computed once and reused by the profile and policy branches)
- [x] No security issues introduced (fails closed; the error names only config)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
