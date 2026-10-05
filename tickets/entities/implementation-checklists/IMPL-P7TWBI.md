---
id: IMPL-P7TWBI
type: implementation-checklist
title: 'Implementation: Tracer and analyze skip faced types'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (tracer/faces_test.go, analysis/faces_analyze_test.go, visibility/facegate_test.go TestVisibleTracer_HiddenFace)
- [x] Integration tests written (test full flow, not just units) (cli/analyze_face_test.go, mcp/analyze_face_test.go, dataentry/analyze_face_test.go through the router with an ACL principal)
- [x] Happy path implemented
- [x] Edge cases from planning handled (hidden face, edge hung from a hidden face, edge on a face with no row, faces of one id are not duplicates)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (memstore seeds and the existing facedApp/gatedServer shapes)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (`rela --project=tickets analyze` before and after)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (mutation: disabling the tail-face check fails the hidden-face trace and path tests)

**Verification Evidence:** `analyze` on the tickets project gives the same
finding sets as the base binary for all seven checks. Orphans and duplicates are
now sorted; JSON gains a `coverage` field, and the orphan entries are `{id,
type, title, faces}`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (tracer.FoldOrphans and tracer.NodeOf are shared by the generic tracer and the visibility decorator)
- [x] No security issues introduced (gate before fold; hidden faces and their edges are absent from every face list, trace and path)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
