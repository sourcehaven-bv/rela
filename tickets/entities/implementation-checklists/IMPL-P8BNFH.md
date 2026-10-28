---
id: IMPL-P8BNFH
type: implementation-checklist
title: 'Implementation: Inbox 3: principal mentions and replies'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestClassifySpans`, `TestPost_MentionTargets`, `TestPost_LoginMentions`, `TestPost_RefPropMention`, `TestPost_MentionAnswerIsUniform`, `TestPost_TooManyMentions`, `TestEdit_*` mention cases, `TestPost_ReplyWakesParentAuthor`, `TestRender_Mentions`, `TestInbox_Mention*`, `TestLocal_AddressedOptIn`, `TestLocal_SubscribeWhileMarking`; SPA markdown, channelView, channelErrors and wireFields tests
- [x] Integration tests written (test full flow, not just units): channelstest conformance (AppendAddressed, EditRemoveAddressed, ListAddressed, ListAddressedKind, AddressedTargetCap, AddressedErased) on all stores; sqlite v29 and postgres 0036 migration and EXPLAIN tests; `TestInboxMention_ACL`; MCP `TestListInbox_Mentions` and watch tests; CLI `TestChannelInbox_Mentions`, `TestInbox_JobIdentityMention`; budget tests for post, messages and inbox
- [x] Happy path implemented: mention and reply rows on post and edit, inbox `mention`/`reply` kinds, MCP `list_inbox`, `rela channel inbox`, SPA chips
- [x] Edge cases from planning handled: uniform post answer for hidden targets, 10-target cap, edits keeping unreadable targets, cap eviction order, kind filter inside the 2000-row scan, gate before the cut, job identities, erasure on tombstone
- [x] Error handling in place (errors surfaced, not swallowed): 422 `too_many_mentions` with a channelErrors rule; file-store row failures logged without failing the post

## Test Quality

- [x] Using fixture builders or factories for test data: channelstest harness, realtime service harness
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: SPA chips covered by component tests; flows by handler, MCP and CLI tests)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-GMEUX4
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on all five tags (0
issues); race tests on touched packages; sqlite- and postgres-tag channel suites
with a database; arch-lint, plimsoll, comment-lint, markdownlint clean; coverage
floors pass; `just docs` idempotent; frontend vitest, vue-tsc, eslint.

## Quality

- [x] Code follows project patterns (check similar code): channel store conformance, schema ladders, collection-read budget tests, opt-in live subscriptions
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): shared `channels.AddressedRow` and kind
constants; exported production SQL reused by plan tests
- [x] No security issues introduced: rela-security-reviewer findings (2 minor) fixed or tracked in TKT-9KNO8X
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
