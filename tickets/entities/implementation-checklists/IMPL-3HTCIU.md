---
id: IMPL-3HTCIU
type: implementation-checklist
title: 'Implementation: Anchored documents skip the face gate: a published-only reader renders the draft'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: handler tests drive the real Lua engine and command runner; no UI change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: each edge case is a subtest: denied face, missing entity, bare faced id, command render)

**Verification Evidence:** TestAnchoredDocument_FaceGate on the faced policy
fixture. Before the fix every ID@face render answered 500 and alice
(policy@published) reached the renderer for POL-1@draft. After it alice gets the
uniform 404 on POL-1@draft (same body as a missing entity) and 200 with the
published title on POL-1@published; bob renders the draft on POL-1@draft through
Lua and through a command renderer. `go test ./internal/dataentry/...` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
