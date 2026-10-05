---
id: IMPL-8GQUGV
type: implementation-checklist
title: 'Implementation: Relation reads and writes mishandle faced endpoints'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: server-side payload and edge filtering with no UI change; the gantt case is unreachable until faced sources load in Stage 2, so it is pinned at the edge filter)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Already fixed by the TKT-2528AB PRs, confirmed by reading the code and its
tests:

- `FilterRelations` gates through `EndpointsReadable`: `internal/visibility/endpoints_test.go` (`TestEndpointsReadable`, `TestPolicyReader_FilterRelationsBudget`).
- Relation create and GET to a faced target, and the clone and relation-write oracle: `internal/dataentry/relation_faces_test.go` (`TestRelation_FacedHeadByBareID`, `TestRelationWrites_DeniedFaceIsTheUniformMiss`, `TestRelationGet_EdgeOnAnotherFaceIsNotFound`).
- MCP show drops another face's content edges (`internal/mcp/convert.go`).

Fixed in this PR, each test failing without the fix:

- `internal/dataentry/commands_face_test.go`: `TestCommandEntityInput_ContentEdgesOnlyWithTheirFace`, `TestCommandEntityInput_PeerGated`, `TestCommandViewInput_ContentEdgesOnlyWithTheirFace`, `TestCommandViewInput_IDAtTwoFaces`, `TestCommandInputs_ReadFaultFailsTheBuild`, and the `storetest.Counting` budget `TestCommandInputs_RelationReadBudget` (same reads at 10 and 50 edges).
- `internal/dataentry/gantt_face_test.go`: `TestGanttEdges_ContentEdgeOnlyWithItsFace`, `TestGanttNodes_KeepTheirFace`, `TestGanttSubtree_DeclinesForFacedSources`.
- `internal/visibility/endpoints_test.go`: `TestEndpointsReadableErr_ReturnsFaults`.

`go test ./internal/dataentry/... ./internal/visibility/...
./internal/archguard/...`, `golangci-lint`, `just arch-lint`, `just
comment-lint` and `just plimsoll` pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
