---
id: IMPL-NXOZFA
type: implementation-checklist
title: 'Implementation: Server-known quiet message types (unread, inbox)'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestParse_Unread`, `TestCheckReload_UnreadFlip`, `TestPost_NoUnreadType`, `TestTriggerPost_NoUnreadType`, `TestReminderStep_NoUnreadType`, `TestReminderMessage_CopiesNoUnread`, `TestRender_NoUnreadNotOnWire`, `TestCounts`, `TestRealtimeChannelSchema_MessageType`, `useMessageLink.test.ts`
- [x] Integration tests written (test full flow, not just units): `channelstest` `ReadsNoUnread` and `ReminderNoUnread` on file, sql, sqlite and postgres; `TestInbox_UnreadSkipsNoUnread`, `TestInboxSummary_NoUnreadType`, `TestLiveInbox_NoUnreadPost*`, `TestValidate_HiddenCountsUnread`, `ChannelsSection.deeplink.test.ts`
- [x] Happy path implemented: `unread: false` on a stored message type keeps its posts out of unread counts and the badge; inbox mention and reply rows link with `&seq=` and the channel reveals that message once
- [x] Edge cases from planning handled: all three write paths (person, trigger, reminder); refused on signal types and for non-boolean values; seq-only change, cleared after use, tied to the linked channel, bad seq ignored; hidden-but-counting warning skips signal types
- [x] Error handling in place (errors surfaced, not swallowed): load errors name the type and key; a failed page load shows the channel's usual error

## Test Quality

- [x] Using fixture builders or factories for test data: `channelstest` harness, live-inbox env, realtime config fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the deep link is covered by component tests with the router; the server paths by store conformance and live tests)
- [x] Each acceptance criterion verified with test scenario from planning: AC table in the notes maps 1 to 12 to tests
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build on four tags; vet; `just lint` (0 issues on all
tag sets), arch-lint, plimsoll, comment-lint; `go test -race` on touched
packages with channels, realtime and appbuild also under postgres and sqlite;
dataentry under race; 5447 frontend tests, type check, lint, `just
build-frontend`; `just docs`; markdownlint; archplan ci; coverage check.

**Deviations:** `ChannelSchema` has three methods (two old ones lost their only
caller); the link-reveal logic is in `useMessageLink.ts` for the file-size lint;
the tail-cost bound is three times the newest segment because the reader doubles
its window; deep-link tests mock the router like the existing cases.

## Quality

- [x] Code follows project patterns (check similar code): copies the `notify: false` opt-out shape; strict boolean via `internal/yamlscalar` like `confirm`; reuses the counts column and partial index; reuses the reply-quote reveal
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `MessageType` lookup replaces two
schema methods; one composable for the link reveal
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
