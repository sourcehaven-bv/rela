---
id: IMPL-SJYXHEE
type: implementation-checklist
title: 'Implementation: Inbox: answer waiting sessions from the inbox row'
started: "2026-11-03"
completed: "2026-11-03"
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

- Store: conformance tests `SessionCause`, `SessionCauseKept`, `GetMessages`,
`GetMessagesRefusals`, `GetMessagesMax` pass on file, sqlite and postgres (local
Postgres.app, `-tags postgres`). Damaged rows: `TestGetMessages_Damaged` (file),
`TestGetMessages_DamagedRow` (sql). sqlite plan: `TestGetMessages_UsesIndexes`.
- Migrations: sqlite v31 to v32 (`TestMigrate_v31ToV32SessionCause`), postgres
0039 (`TestMigration_SessionCause`); renumbered after develop took v31 and 0038
for retired ids (TKT-J6GISC3).
- Realtime: `TestInbox_WaitingMessage` (ask, newer messages, removed for a
reader and for a moderator), `TestInbox_WaitingWithoutCause`,
`TestInbox_WaitingMessageRefs`, `TestInbox_DecisionInputs`,
`TestWaitingMessages_Refusals`, `TestInbox_WaitingMessagesBudget` (10 and 50),
`TestInbox_WaitingMessageHiddenRows`.
- HTTP: `TestInbox_DecisionInputsMatchHead` (real ACL; actions, transitions,
cause, last_seq and message equal the head and the message list),
`TestInbox_WaitingMessagesHiddenRowsRealACL`, `TestInboxBudget_WaitingMessages`
(one GetMessages, no Get, Head or Messages, same cost at 10 and 50),
`TestInboxSummary_NoWaitingMessages`. `TestLive_InboxRoundMemo` unchanged.
- AC 7 benchmark `BenchmarkLive_InboxRound` (M1 Pro): file store with
`RELA_BENCH_INBOX_FILL=700` (700 messages of 1 KB before a 4 KB cause in each of
500 cases): 2.37 s per round, grid 4 s. PostgreSQL with fill 50: 31 ms per
round, target 500 ms. The file store meets its grid with full segment parses, so
the seek fallback is not needed. The fill is opt-in because seeding 350 000
messages on the file store takes about 108 minutes.
- SPA: unit tests for `InboxWaitingItem`, `inboxDecisionHead`,
`decisionGlue`, `OpenChannel.start`, `InboxView`, `channelView`,
`useSessionDecisions`; RlInbox stories. E2E scenario "the owner answers
questions and approves a plan from the inbox" passes.
- MCP `list_inbox` and `rela channel inbox` output unchanged: the inbox-page
fields are `json:"-"` in realtime; mcp and cli tests pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
