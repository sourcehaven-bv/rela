---
id: IMPL-FTK20N
type: implementation-checklist
title: 'Implementation: Write responses name neighbours the caller cannot read'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) (handler-level tests through the real ACL, store and automation engine)
- [x] Happy path implemented
- [x] Edge cases from planning handled (visible neighbour stays in the response)
- [x] Error handling in place (errors surfaced, not swallowed) (gate errors go through writeGateError)

## Test Quality

- [x] Using fixture builders or factories for test data (hiddenNeighborApp)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: response-shape fix fully covered by handler tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** Before the fix all three
Test*_ResponseHidesUnreadableNeighbor tests failed with
`"relations":{"implements":["FEAT-0","SEC-1"]}`; after it they pass, and go test
./internal/dataentry/... ./internal/visibility/... passes.

## Quality

- [x] Code follows project patterns (check similar code) (same seam as the GET and PATCH responses)
- [x] Checked for DRY opportunities (reuses servedFaceEdges; no new helper)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
