---
id: IMPL-E6Z310
type: implementation-checklist
title: 'Implementation: Built-in typing indicator per channel type'
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg` `TestParse_Typing*`, `TestParse_BoolKeysUnchanged`, `TestCheckReload_TypingToggleAllowed`; `realtime` `TestTyping_*`, `TestRead_Typing`, `TestTypingStore_StaysInItsChannel`; frontend `useTyping.test.ts` and `typingViews` tests
- [x] Integration tests written (test full flow, not just units): dataentry `TestChannelTyping_*`, `TestChannelSchema_Typing`, `TestLive_TypingFrames` (fake clock), `TestLive_TypingReloadRound`, `_typing` rows in route/404/parse tables; `TestChannelPaths_Typing`; MCP `TestReadChannel_Typing*`; e2e "a reader sees who is typing until they send"
- [x] Happy path implemented: `typing: true` on a channel type; the composer sends typing throttled; other readers see "<name> is typing…" until expiry or send
- [x] Edge cases from planning handled: hidden anchor 404, unstored named channel 404 without creating it, closed channel, reload toggling typing off, entity delete/rename cleanup, self-exclusion across tabs, caps refuse without evicting, in-flight typing settled before post
- [x] Error handling in place (errors surfaced, not swallowed): 403 `typing_not_offered`, 403 `channel_post_denied`, 409 `channel_not_open`, 429; the SPA backs off silently by design (typing is best effort)

## Test Quality

- [x] Using fixture builders or factories for test data: dataentry test project `chat` and `briefing` types; `appbuildtest.WithChannelsLocal` for the clock
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `channels.spec.ts` (6 tests) passed in a real browser
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-4G509O
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; `go test -race` on
realtime, realtimecfg, dataentry, appbuild, openapi, mcp; sqlite-tag tests on
realtime, dataentry, mcp; golangci-lint 0 issues on all tags; arch-lint,
comment-lint, plimsoll clean; vitest 4676 tests, vue-tsc (app and library),
eslint, 37 story tests; `just coverage-check` 84.9%; `just docs` idempotent.

## Quality

- [x] Code follows project patterns (check similar code): reuses the post-role check, rate limiter, head and live marks; consumer-side typist interface
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one split helper for Head and Read;
constants in realtimelimits
- [x] No security issues introduced: rela-security-reviewer found one minor (full scan under the lock), addressed
- [x] No silent failures (errors logged AND returned): typing errors in the SPA are deliberately silent (best effort); server errors are returned
- [x] No debug code left behind
