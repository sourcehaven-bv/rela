---
id: IMPL-3WOPRE
type: implementation-checklist
title: 'Implementation: Sessions 4/2: session UI'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `internal/realtime/session_affordance_test.go` (descriptor, transitions per caller and state, subset of post, agreement with real posts, roles asked once, first post by creator); vitest for `channelView`, `useChannel`, `channelSchema`, `ChannelsPanel`; story play tests for the stepper, notices, composer, channel and segmented control
- [x] Integration tests written (test full flow, not just units): dataentry schema, budget and wire golden tests; OpenAPI parity; MCP `read_channel`; e2e session scenario in `channels.spec.ts`
- [x] Happy path implemented: the channel header shows the session stepper; the composer lists transition types first with a hint; a move renders as a one-line notice and is announced
- [x] Edge cases from planning handled: unset (initial), re-entry, final (Ended), stranded, closed (Paused), stale descriptor (Loading), narrow layout, single offered type, reorder with focus kept
- [x] Error handling in place (errors surfaced, not swallowed): 409 `session_notice`/`session_message` refetch; a failed descriptor reload keeps the Loading state

## Test Quality

- [x] Using fixture builders or factories for test data: the breach-room fixture in rela-components and the docs-example session fixture in realtime tests
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `channels.spec.ts` (7 tests) in a real browser, including the session scenario
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-94ZZ7L
- [x] ~~Edge cases manually verified~~ (N/A: covered by stories and tests)

**Verification Evidence:** build, vet and golangci-lint on all four tags; `go
test -race` on realtime, dataentry, mcp, openapi, apiwire, cli; sqlite-tag tests
on realtime, dataentry, mcp; arch-lint, comment-lint, plimsoll clean; app vitest
4710, vue-tsc and eslint; library vue-tsc, `npm run check` and the touched story
tests; e2e channels 7/7; `just coverage-check` 84.9%; `just docs` idempotent.

## Quality

- [x] Code follows project patterns (check similar code): `_actions` affordance contract, the shared role check of the post path, rela-components for presentation and the app for wiring
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `heldRole` lookup shared by post and
transitions; `isSessionNotice` reused
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
