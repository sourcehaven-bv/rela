---
id: IMPL-2OBRPS
type: implementation-checklist
title: 'Implementation: Family delete authorizes one face and deletes all faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestFamilyDelete_DeniedUnlessEveryFaceIsDeletable, TestFamilyDelete_EveryFaceCapturedAndAudited)
- [x] Integration tests written (test full flow, not just units) (real acl.Declarative policy with per-face delete grants, run over memstore, fsstore, sqlite and postgres)
- [x] Happy path implemented
- [x] Edge cases from planning handled (either face denied; face added between check and delete is re-authorized inside the Tx and after the store scan)
- [x] Error handling in place (errors surfaced, not swallowed) (family read errors fail closed; TestDelete_FailsClosedOnNonNotFoundFetchError updated to fail ListEntities)

## Test Quality

- [x] Using fixture builders or factories for test data (newFamilyDeleteFixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: CLI delete pre-checks with the zero-face GetEntity and reports a faced entity as not found before reaching the manager, a separate face-blind surface; covered by the backend-matrix tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (postgres run with RELA_TEST_DATABASE_URL, sqlite with -tags sqlite)

**Verification Evidence:** go test ./internal/entitymanager/...
./internal/mcp/... ./internal/lua/... ./internal/cli/...
./internal/dataentry/... passes. `-tags sqlite` and `-tags postgres` runs of
TestFamilyDelete_* pass on every backend.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors authorizeCascadeRelations in-Tx re-check and DeleteEntityFace audit summary)
- [x] Checked for DRY opportunities (one authorizeFamilyDelete helper used at all three points)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
