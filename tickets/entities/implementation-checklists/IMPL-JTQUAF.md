---
id: IMPL-JTQUAF
type: implementation-checklist
title: 'Implementation: Deleting a face with a content-scoped edge is refused: relation check reads the zero face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (faceedge_delete_acl_test.go: TestDeleteEntityFace_ContentEdgeAuthorizedByItsTail, TestDeleteEntityFace_ContentEdgeDeniedWritesNothing, TestDelete_CascadeResolvesEdgeSourceType, TestDeleteEntityFace_SourceReadErrorAborts)
- [x] Integration tests written (test full flow, not just units) (real acl.Declarative policy over memstore, fsstore and sqlite; e2e face-delete spec in faces-backlog.spec.ts is live)
- [x] Happy path implemented
- [x] Edge cases from planning handled (identity edge or missing tail face falls back to any face; unresolvable source keeps "" and fails closed; face delete, family delete and faceless-target cascade)
- [x] Error handling in place (errors surfaced, not swallowed) (a store error while resolving the source aborts the delete instead of reading as a denial)

## Test Quality

- [x] Using fixture builders or factories for test data (newFaceEdgeFixture, policyFace)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (Playwright: faces-backlog "deleting the draft face removes its edges and keeps the published face" passes against the built rela-server)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (sqlite with -tags sqlite; postgres not run, RELA_TEST_DATABASE_URL unset)

**Verification Evidence:** go test ./internal/entitymanager/...
./internal/archguard/... passes; -tags sqlite passes on every backend; npx
playwright test faces-backlog.spec.ts faces-write.spec.ts: 7 passed, 14 skipped
(fixme).

## Quality

- [x] Code follows project patterns (check similar code) (reuses anyFaceOf as the relation write paths do; reads through the Tx view)
- [x] Checked for DRY opportunities (one edgeSourceType helper)
- [x] No security issues introduced (still fails closed when the source resolves to nothing)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
