---
id: IMPL-E11ZPY
type: implementation-checklist
title: 'Implementation: Entity export 404s for every faced address, while the detail page still offers the Export menu'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`internal/dataentry/export_face_test.go`, two Vitest cases in `EntityDetail.world.test.ts`)
- [x] Integration tests written (test full flow, not just units): the export and document tests go through `app.NewRouter()`, so world middleware, routing and the real `cp` transform run.
- [x] Happy path implemented: `ID@face` exports that face with the built-in renderer and with `export_render`.
- [x] Edge cases from planning handled: unreadable face gives the missing-entity 404; the bare id stays the bare face (export is not world-capable); content-scoped edges come from the exported face only; faced anchored documents render and export.
- [x] Error handling in place (errors surfaced, not swallowed): a neighbor-resolution fault is a gate error, not an empty relation list (RR-4TFZNL).

## Test Quality

- [x] Using fixture builders or factories for test data (`facedApp`, `seedPolicyFaces`, `withRenderOverride`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by router-level HTTP tests with a real transform; not run in a browser)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- AC1: `TestExport_FacedAddressExportsThatFace` (explicit faces, and a bare id
resolved by the default world),
`TestExport_FacedRenderOverrideReceivesTheAddress`.
- AC2: `TestExport_UnreadableFaceIsIndistinguishableFromMissing` (body identical
to a missing entity apart from the echoed path).
- AC3: the TKT-5SZG2L comment is replaced; the new comment cites BUG-PLZDPR.
- Anchored documents: `TestDocument_FacedAnchorRendersThatFace` (render and
export); face gate in `TestFaceGrant_AnchoredDocumentIsFaceGated` (BUG-6DBV6N).
- Worlds: `TestExport_FacedNeighborsResolveInTheWorld`,
`TestExport_DeniedWorldIsNotFound`, `TestExport_NeighborFaceGateHidesTheLink`,
`TestExport_NeighborFaultFailsTheExport`; `TestWorldCapablePath` rows.
- SPA: Vitest `exports the served ADDRESS in the page world, not the route id`.
- Mutation checks: reverting `export.go` fails the export tests; reverting
`document.go` fails the document test; removing the export world admission fails
five export tests; forcing every neighbor head visible fails the neighbor
face-gate test; disabling the document face gate fails its test; reverting the
SPA URL fails the Vitest case.

## Quality

- [x] Code follows project patterns (check similar code): late-bound `faceNeighbors` closure as in `newViewsHandler`; `isSafeStateRefSegment` as in `resolveAnchoredDocument`.
- [x] Checked for DRY opportunities: `loadEntry` replaces two identical bare-id reads in `document.go`.
- [x] No security issues introduced: the read goes through `getVisibleRef` (row gate, face gate, denied world) and is redacted once before rendering; neighbor titles come from world-resolved, gated heads, each redacted once.
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
