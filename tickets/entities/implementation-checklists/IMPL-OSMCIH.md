---
id: IMPL-OSMCIH
type: implementation-checklist
title: 'Implementation: Entity-anchored documents skip the face gate: a face-restricted reader can render a hidden face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestFaceGrant_AnchoredDocumentIsFaceGated`
- [x] Integration tests written (test full flow, not just units): the test drives `handleV1Documents` for the HTML and export routes
- [x] Happy path implemented: the granted face renders (positive control)
- [x] Edge cases from planning handled: the denied face on both routes; the renderer must not run
- [x] Error handling in place (errors surfaced, not swallowed): a denied face answers the uniform entity 404

## Test Quality

- [x] Using fixture builders or factories for test data: `facedApp`, `withFacedReportDoc`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by handler-level HTTP tests; not run in a browser)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `TestFaceGrant_AnchoredDocumentIsFaceGated` passes, and fails with the
`faceReadable` check disabled: both routes return 200 with the draft render, and
the fake engine records the call.

## Quality

- [x] Code follows project patterns (check similar code): same `faceReadable` check the entity GET and attachments use
- [x] Checked for DRY opportunities (none: one call to an existing helper) — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced: the change only narrows access
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
