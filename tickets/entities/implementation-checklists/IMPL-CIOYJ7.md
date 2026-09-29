---
id: IMPL-CIOYJ7
type: implementation-checklist
title: 'Implementation: Content-scoped edges are served on faces that do not own them'
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
<!-- Document what you tested and the results -->

Before the fix, a rela-server over the e2e faced project served POL-1@draft's
content-scoped `implements` edge on `_views/policy/POL-1@published` and as an incoming neighbor of
CTL-1 in the published world, for an editor and a published-only reader. After
the fix, the same requests serve only the published face's edge and the identity
edge. `contentedge_face_test.go` covers both readers through the router; with
the fix reverted it fails on views, include, list rows, the relation filter,
table columns and export.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
