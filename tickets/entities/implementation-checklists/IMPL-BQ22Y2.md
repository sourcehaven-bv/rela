---
id: IMPL-BQ22Y2
type: implementation-checklist
title: 'Implementation: MCP tools for running actions'
started: "2026-10-11"
status: done
completed: "2026-10-11"
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`internal/mcp/tools_actions_test.go`,
`internal/dataentry/actions_mcp_test.go`)
- [x] Integration tests written (test full flow, not just units): the action
runs through the real router and the remote MCP mount with a JWT principal
(`TestMCPActions_ThroughTheRemoteMount`,
`TestProvision_ActionProvisionsOnBothTransports`)
- [x] Happy path implemented
- [x] Edge cases from planning handled: confirm, request body cap, body for an
action without `request:`, unreadable and absent entities, reload
- [x] Error handling in place (errors surfaced, not swallowed): every refusal
is a `script.ActionError` with the HTTP code

## Test Quality

- [x] Using fixture builders or factories for test data (`newDetailActionApp`,
`newActionTestApp`, `provisionApp`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: through the real router with the JWT
gate and the remote MCP mount, as an integration test
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1, AC2: `TestMCPActions_List` (unbound with and without permission,
  entity-bound, when false, absent and unreadable entity).
- AC3: `TestMCPActions_Responses` (envelope and rich), `TestActionTools_Run`.
- AC4: `TestMCPActions_RunGates`, `TestMCPActions_ConfirmAfterGates`,
  `TestMCPActions_ConfigReloaded`, `TestMCPActions_ThroughTheRemoteMount`.
- AC5: `TestMCPActions_ConfirmRequired`.
- AC6: `TestMCPActions_Responses` (body over the cap, body without `request:`).
- AC7: `TestMCPActions_Responses` (script error is degraded),
  `TestActionTools_Run` (refusal carries the code).
- AC8: `TestActionTools_Registration`, `TestRemoteMCPDeps_PassesTheHostActions`.
- AC9: all existing HTTP action tests pass unchanged.

## Quality

- [x] Code follows project patterns (check similar code): mirrors the comment
tools from TKT-QIQO1W
- [x] Checked for DRY opportunities: the failure type is shared with the
comment operations (`opFailure`)
- [x] No security issues introduced: same gates as the HTTP route; degraded
script detail only
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
