---
id: IMPL-ULCMSIB
type: implementation-checklist
title: 'Implementation: MCP update_entity ignores field-level write grants'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestBuildFieldWriteGate)
- [x] Integration tests written (test full flow, not just units) (TestRemoteMCP_UpdateEntityHonorsFieldGrants drives update_entity over HTTP through appbuild.New and RemoteMCPServer; the golden corpus)
- [x] Happy path implemented
- [x] Edge cases from planning handled (nil, NopACL and ReadOnlyACL keep AllowAllFieldGate; the gate follows the resolved ACL)
- [x] Error handling in place (errors surfaced, not swallowed) (a policy that fails to compile panics the fixture, like its other build steps)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (production test plus a mutation check: swapping the MCP handle for the ungated manager makes it fail)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (every golden diff checked by hand)

**Verification Evidence:**

- TestRemoteMCP_UpdateEntityHonorsFieldGrants passes; it fails with "update_entity wrote note" when internal/dataentrywire/mcp.go uses svc.EntityManager().
- Re-recorded corpus: decl-jwt/BUG-VUNO11-mcp-update-denied-field.editor.golden now reads `field-affordance:read-only:note`, matching REST update-denied-field (403, same rule id). The admin recording gains one GetRelation from the gate; its body is unchanged. No other recording changed.
- go test passes for internal/dataentry (full package), internal/dataentrywire, internal/mcp, internal/appbuild, internal/appbuild/appbuildtest.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors BuildTransitionWiring)
- [x] Checked for DRY opportunities (the fixture reuses buildFieldPolicy instead of a second gate path)
- [x] No security issues introduced (rela-security-reviewer: no critical or significant findings)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
