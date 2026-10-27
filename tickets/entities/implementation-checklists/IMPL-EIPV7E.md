---
id: IMPL-EIPV7E
type: implementation-checklist
title: 'Implementation: Inbox 2: read markers and unread counts'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `channels` read-state checks and `ClampRead`; `realtime` `TestMarkRead_*`, `TestInbox_Unread*`, `TestInbox_Kinds`, `TestInboxView_MarshalJSON`, `TestLocal_MarkForScopesReader`; CLI `TestChannelMarkRead_Text`, `TestChannelInbox_Unread`; MCP `TestMarkRead_Tool`, `TestReadChannel_DoesNotMoveMarker`; SPA `useMarkRead` tests
- [x] Integration tests written (test full flow, not just units): channelstest conformance for SetRead, Reads, ListReads, own-post marks and LastAt on all four stores; sqlite v28 and postgres 0035 migration tests; EXPLAIN tests on the production statements; `TestChannelMarkRead_Handler`, `TestInboxUnread_ACL`, live read frames and inbox watch wakes
- [x] Happy path implemented: read markers, unread counts in channel list, head and inbox, `PUT .../_read`, MCP `mark_read`, `rela channel mark-read`, SPA badges
- [x] Edge cases from planning handled: monotonic and clamped markers, own posts, removed messages and notices, file-store write failure after a post, cap at 100, 500-marker window ordered by activity, kinds with cursor and the 503 ceiling, job identities, read-only clients
- [x] Error handling in place (errors surfaced, not swallowed): typed denials; file-store marker failures logged without failing the post

## Test Quality

- [x] Using fixture builders or factories for test data: channelstest harness, realtime service harness, testdata policy
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: SPA badges covered by component tests; flows by handler and wiring tests)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-LEN9V3
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** golangci-lint on all five tags (0 issues); race tests
on touched packages; sqlite- and postgres-tag tests with a database (known
TestGoldenParity pile goldens excepted); arch-lint, plimsoll, comment-lint,
markdownlint clean; coverage floors pass; `just docs` idempotent; frontend
vitest, vue-tsc, eslint, prettier.

## Quality

- [x] Code follows project patterns (check similar code): channel store conformance, schema ladders, collection-read budget tests, live hub replay per subscriber
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): shared `channels.Counts` and `ClampRead`;
exported production SQL reused by plan tests
- [x] No security issues introduced: rela-security-reviewer finding (1 minor) documented, erasure in TKT-6V8K8I
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
