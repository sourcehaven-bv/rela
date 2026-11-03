---
id: IMPL-RVFWG8
type: implementation-checklist
title: 'Implementation: Client field ceilings are ignored when no role declares field grants'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestPolicy_HasAffordanceGrants_ClientFieldCeiling, ceiling key rejection cases, TestMapWebhook_CheckMapping cases)
- [x] Integration tests written (test full flow, not just units) (TestCeilingOnlyPolicy_RedactsField over appbuild reads and search; TestCeilingE2E_FieldCeilingWithoutRoleFieldGrants over the HTTP router; TestWebhookCeiling_MapAgreesWithRuntime with real deliveries; CLI exit-code test)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty visible list, row-only baseline, scope grant alone, wildcard and faced keys, find-only hook, unreadable find type)
- [x] Error handling in place (errors surfaced, not swallowed) (invalid ceiling keys fail policy load with a named error)

## Test Quality

- [x] Using fixture builders or factories for test data (existing helpers: writeMetamodelBody, writePolicy, newTestAppV1, hookWorld, hookServices)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Both new integration tests fail on develop and pass
with the fix. Ran the built `rela acl map --webhook alert` against a temp
project whose baseline redacts the match field: the report lists `match fields
hidden by the client ceiling: host` and prints `mapping check failed`. `go test
./...` passes except the known local-only cmd/rela-desktop TestChromeStyle
failure (unbuilt frontend).

## Quality

- [x] Code follows project patterns (check similar code) (load-time rejection mirrors the existing type@face rule in validateTypeOnlyEntries)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (rela-security-reviewer: no critical or significant findings in the diff)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
