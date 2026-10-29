---
id: IMPL-2711RP
type: implementation-checklist
title: 'Implementation: Hide agent-internal message types from people'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestChannelsPresentation_Validate` display cases, `TestValidateChannels_Display`, `TestAllowlist_ClassifiesEveryKey`, `TestCheckEdit_DataEntry`; SPA `channelFold.test.ts` table, `channelView.test.ts` "passes display", the `RlButton` Toggle story
- [x] Integration tests written (test full flow, not just units): `TestConfig_ServesChannels`; `ChannelsPanel quiet types` (decision cause, choices, reply quotes on the same and an older page, late announcement, focus); stories "Quiet types", "Quiet types, scrolled up", config reload
- [x] Happy path implemented: `channels.<type>.messages.<mt>.display: collapsed | hidden` in data-entry.yaml; adjacent collapsed messages fold into one line; hidden ones are left out; "Show all messages" toggle; Configure may edit the key
- [x] Edge cases from planning handled: kept rows (decision cause, choices, actions, reply targets) stay kept for the visit; fold keyed by first seq; scroll kept on toggle, expand, reveal and config reload; quote reveal on an older page; announcements follow what renders; empty-log hints; removed messages in runs
- [x] Error handling in place (errors surfaced, not swallowed): a bad `display` value is refused at load and in Configure with its path; an unknown value in the SPA shows the message in full

## Test Quality

- [x] Using fixture builders or factories for test data: channel fixtures, panel harness, story fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no e2e in the plan; the Storybook interaction stories run in a real browser)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-0RKHWQ and the notes table
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint,
arch-lint, plimsoll, comment-lint clean; `go test -race` on dataentryconfig,
configedit, dataentry and cmd/rela-server; markdown corpus round trip; frontend
typecheck, eslint (no new warnings), vitest (5314 tests); library vitest (637
tests), `vue-tsc -b` and Storybook build; markdownlint; `just docs`. Mutation
checks confirm the key tests fail without the fixes.

**Deviations:** scroll helpers moved to `channelAnchor.ts` to keep
`RlChannel.vue` under max-lines; `shownAs` instead of `rendersInFull`; fixed
text keys for the toggle and empty hints; `RlButton` gained a pressed style for
every toggle; the docs example test reads `docs/data-entry.md`.

## Quality

- [x] Code follows project patterns (check similar code): presentation keys next to `fields` in the `channels:` block; allowlist entry beside the `fields` line; toggle like other `aria-pressed` buttons
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one fold builder (`channelFold.ts`); one
shared `CHANNEL_OPERATIONS` list; message binding written once
- [x] No security issues introduced: rela-security-reviewer found none; docs state this is not access control
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
