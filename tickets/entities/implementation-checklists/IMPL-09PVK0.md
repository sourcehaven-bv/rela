---
id: IMPL-09PVK0
type: implementation-checklist
title: 'Implementation: Gaining a role can narrow field visibility and writability'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (roleunion_test.go, roleunion_dims_test.go, rolegrant_test.go, A14 audit test, dataentry create routing test)
- [x] Integration tests written (golden wire-parity corpus runs the HTTP handlers, MCP and history end to end; 9 recordings regenerated and reviewed)
- [x] Happy path implemented
- [x] Edge cases from planning handled (face-qualified grants, ceiling, unstamped everyone, history, create vs update verb, relation meta)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newUnionWorld, newRoleUnionWorld)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via the golden corpus: real HTTP and MCP requests against the fixture policy)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (mutation check: disabling the block-less contribution makes TestRoleUnion_Monotonic report 1744 violations)

**Verification Evidence:**
- `go test -race` on affordances, acl, aclaudit, entitymanager, visibility, mcp, lua, cli, appbuild, dataentry, archguard: pass.
- Golden diffs: editor (reads ticket, no visible: block) now sees budget/note on watched TKT-4 rows and the full history snapshot; stranger (asserted editor) likewise in history. Viewer recordings unchanged.
- golangci-lint, arch-lint, comment-lint, plimsoll: clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (WriteGate.check and checkFieldWrite share the update and create paths)
- [x] No security issues introduced (ceiling applied before the verb check and after the union; history uses globals only)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
