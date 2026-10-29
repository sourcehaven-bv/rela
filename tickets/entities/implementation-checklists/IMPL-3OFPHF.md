---
id: IMPL-3OFPHF
type: implementation-checklist
title: 'Implementation: Session decisions as one-click buttons'
started: "2026-10-28"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestPost_ExpectGeneration`, `TestPost_ExpectGenerationStoreRefuses`, `TestPostMessage_ExpectGeneration`, `TestChannelCLI_ExpectGeneration`, `TestChannelPaths_ExpectGeneration`; SPA `decisionViews`, `decisionCause`, `decisionReplyTo` tables, `useSessionDecisions.test.ts` (12 tests), `useChannel` `actionsGeneration` and `refreshHead`, `channelErrors` `session_moved`
- [x] Integration tests written (test full flow, not just units): `SessionAdmitGeneration` conformance on filechannels, sqlchannels, sqlitechannels and pgchannels; `TestChannelsHandler_ExpectGeneration`; ChannelsPanel decision suite; e2e "the lead decides with one click" in `channels.spec.ts`
- [x] Happy path implemented: decision buttons in the session header and on the deciding message, one-click posts with `expect_generation` and `reply_to`, form decisions open the composer with the type selected
- [x] Edge cases from planning handled: 201 that moves nothing, live frame before the POST answer, failed head refresh, channel closes or switches during the request, cause message removed or not loaded, same state re-entered during the confirm dialog
- [x] Error handling in place (errors surfaced, not swallowed): refusals map by problem type (`session_moved`, `session_wrong_state`, `invalid_message`, `rate_limited`, network); typed `SessionGenerationError` from the store

## Test Quality

- [x] Using fixture builders or factories for test data: realtime and channel store harnesses, panel test fixtures, e2e `RoomOptions.decisions`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `channels.spec.ts` 9 of 9 locally
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-GPDE8G
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint 0
issues; arch-lint, plimsoll, comment-lint clean; `go test -race` on channels,
realtime, dataentry, mcp, cli, openapi, apiwire, docs; conformance with sqlite
and postgres tags; frontend vue-tsc, eslint, vitest (5190 tests); library vitest
(610 tests), vue-tsc and Storybook build; markdownlint; `just docs`.

**Deviations:** only one-click decisions ask for confirmation (a form decision
posts through its own Send); `ChannelDecisionView` uses `danger` like
`ChannelActionView`; the e2e review state has no `waiting_for`, which the server
refuses for a permission-held role. Two archplan findings (wire mirror
`PostMessageRequest`/`PostInput`, problem-type strings) accepted in the baseline
with reasons.

## Quality

- [x] Code follows project patterns (check similar code): the precondition reuses `SessionStep.Admit` from TKT-QYFLRH; errors through `classifyChannelError`; library component with stories and `useMessages`
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `decisionViews` mapping feeds header
and row; one `admitPost` for `in:` and generation
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
