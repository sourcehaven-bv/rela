---
id: IMPL-3BNKCB
type: implementation-checklist
title: 'Implementation: Client baseline drops a role''s face grants when the role also holds "*"'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestClamp_WildcardKeepsFaceGrantsTheCeilingPermits`, `TestClamp_IsExactIntersection`)
- [x] Integration tests written (`TestAuthorizeWrite_CeilingKeepsFaceGrant` drives the write gate for create/update/delete)
- [x] Happy path implemented
- [x] Edge cases from planning handled (allowlist "*", type outside the allowlist, deny ceiling, other face, default face)
- [x] ~~Error handling in place~~ (N/A: pure list computation, no error paths)

## Test Quality

- [x] Using fixture builders or factories for test data (shared policy constant, `mustPolicy`, `mustDeclarative`, `verifiedClient`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object~~ (N/A: assertions compare grant lists and verdicts)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Atlas `verify/mcp-baseline/verify.sh` (worktree on
feat/icinga-webhook-baseline, /bin/bash): fails with "agent lost beleid create
(concept writing)" against a develop build, passes against the fixed build.
Without the fix, `TestClamp_IsExactIntersection` reports 48 violations.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (the wildcard branch now shares the per-entry filter loop instead of returning early)
- [x] No security issues introduced (face entries kept only when the ceiling permits their type, the same rule as the non-wildcard branch; exhaustive test checks no over-grant)
- [x] ~~No silent failures~~ (N/A: no error paths)
- [x] No debug code left behind
