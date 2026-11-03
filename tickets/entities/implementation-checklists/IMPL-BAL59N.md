---
id: IMPL-BAL59N
type: implementation-checklist
title: 'Implementation: Inbox 4: SPA inbox page and live updates'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestInboxView_Summary`, `TestInboxSummaryHandler_*`, `TestParseLivePath`, `TestLivePath_InboxCanonical`, `TestInboxClock_TickAt`, `TestSetInboxGrid`, `TestChannelsScan`, `TestLocal_DurableKeys`, `TestSubscribeLive`; SPA `Sidebar.inbox`, `inboxStatus`, `useInboxSummary`, `InboxView`, `RlInbox`, `ChannelsSection.deeplink` tests
- [x] Integration tests written (test full flow, not just units): live hub tests for mention, own post, own mark-read, window topics, session moves, hidden changes, grid timing, dropped ticks, overflow, staggered safety replay and resync; `TestLive_ReplayMatchesGet` inbox cases; budget test for the summary; e2e scenario in `channels.spec.ts` with two browser contexts
- [x] Happy path implemented: `GET /api/v1/_inbox/summary`, inbox live shapes, sidebar Inbox item with badge, inbox page with three sections, deep links, mark read per row, stories
- [x] Edge cases from planning handled: hidden changes send no frame, wake set only from a 200, typing excluded, closed-channel deep links, refusal-only badge clearing, overflow defers instead of framing, wider grid on the file store
- [x] Error handling in place (errors surfaced, not swallowed): summary shares the inbox refusals; the page shows the problem title on load errors

## Test Quality

- [x] Using fixture builders or factories for test data: live hub and realtime harnesses, e2e multi-user support
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e scenario passed 3 of 3 runs locally; `channels.spec.ts` 8 of 8
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-FQNR4A
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on all four tags (0
issues); race tests on touched packages; postgres-tag realtime tests; arch-lint,
plimsoll, comment-lint, markdownlint clean; `just docs` idempotent; frontend
typecheck, eslint, vitest, Storybook build; AC 6 benchmark: 2.04 s per round on
the file store (4 s grid), 86 ms on postgres (2 s grid).

## Quality

- [x] Code follows project patterns (check similar code): live hub replay-and-hash with server-learned topics, Pinia Colada queries, held-content rule
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one per-round inbox memo shared by summary
and page; summary as a projection of the inbox view
- [x] No security issues introduced: rela-security-reviewer findings (1 significant, 1 minor) fixed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
