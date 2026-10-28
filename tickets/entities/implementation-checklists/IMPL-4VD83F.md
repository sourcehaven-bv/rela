---
id: IMPL-4VD83F
type: implementation-checklist
title: 'Implementation: Session composer: offer only message types that apply in the current state'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestParse_MessageIn`, `TestParse_MessageInClone`, `TestParse_MessageInAllowed`, `TestParse_MessageInInvalid` (13 cases), `TestParse_MessageInBrokenType`, `TestMessageInCheck_SkipsMissingType`, `TestAffordances_MessageIn*`, `TestPost_MessageIn`, `TestPost_MessageInEphemeral`, `TestDenialConstructors_HaveTitles`; SPA `channelErrors.test.ts`
- [x] Integration tests written (test full flow, not just units): `SessionAdmit` store conformance on filechannels, sqlchannels, sqlitechannels and pgchannels; `TestPost_MessageInStoreRefuses`, `TestPost_MessageInConcurrent`; `TestSessionTriggerPost_MessageIn`; HTTP `TestChannelsHandler_SessionWrongState`, MCP `TestPostMessage_SessionWrongState`, CLI `TestChannelCLI_SessionWrongState`; e2e session scenario in `channels.spec.ts`
- [x] Happy path implemented: `message_types.<mt>.in:`, affordances and `can_post` follow it, 409 `session_wrong_state` on HTTP, MCP and CLI, SPA refetch with a translated toast
- [x] Edge cases from planning handled: stranded sessions admit only types without `in:`, final states, closed channels answer `channel_not_open` first, trigger posts skipped with a Warn log, edits and typing unaffected, bare `in:` refused at load
- [x] Error handling in place (errors surfaced, not swallowed): typed `*channels.SessionStateError` carries the state the store saw; trigger refusals logged at Warn

## Test Quality

- [x] Using fixture builders or factories for test data: realtime and channel store harnesses, e2e realtime project
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `channels.spec.ts` 8 of 8 locally
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-FUP8IS
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint 0
issues; arch-lint, plimsoll, comment-lint, markdownlint clean; `go test -race`
on channels, cli, dataentry, mcp, openapi, realtime, realtimecfg, appbuild;
channel conformance with sqlite and postgres tags; `just docs` regenerated;
frontend typecheck and vitest (5112 tests); archplan ci reports only the five
known sqlite `entity_search_*` orphan tables.

**Deviations:** the docs example is a bold paragraph read by a new `yamlBlockIn`
helper; `in:` checks live on `messageInCheck` and the session decision in free
functions to stay under the 40-method cap; post bookkeeping moved to
`Local.notePosted` to keep `Post` under the statement limit.

## Quality

- [x] Code follows project patterns (check similar code): `in:` mirrors `session.reminders.<r>.in`; 409 mirrors `lifecycle_wrong_state`; store check through the existing `SessionStep`
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one decision function `sessionRefusal`
shared by the pre-check and the store
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
