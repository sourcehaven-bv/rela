---
id: IMPL-43XVTK
type: implementation-checklist
title: 'Implementation: Agent activity indicator on channels and boards'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestSession_WorkingStates`, `TestBusy_States`, `TestBusy_ReadRole`, `TestBusy_Anchor`, `TestBusy_ManyChannels`, `TestBusy_NotServed`, the busy validation tests, `TestChannelToolDescriptions_Busy`; SPA `RlBusyIndicator.test.ts` (with the minute clock), `KanbanView.cardBusy.test.ts`, ChannelsPanel activity line
- [x] Integration tests written (test full flow, not just units): `TestListBusy_*` through the list handler, `TestListBusy_TwoPrincipals`, `TestLive_BusyList*`, `TestChannelsBudget_ListBusy10vs50`, `TestListBusy_PoCFlow`, the waiting-query name filter in channel conformance; e2e `kanban-busy.spec.ts`
- [x] Happy path implemented: `?busy=<channel type>` adds `_busy` to list rows from one batch per page; kanban `busy:` card field; `RlBusyIndicator` on cards; "Working…" in the session strip; MCP descriptions tell agents to move into a working state
- [x] Edge cases from planning handled: a session that never moved, several channels per anchor (+N), family channels on every face, unreadable rows and channels, a `one` type with stray named channels, config off or wrong (field absent, list still 200), reduced motion
- [x] Error handling in place (errors surfaced, not swallowed): a store error drops `_busy` and keeps the list; config problems refused at load, validate and Configure

## Test Quality

- [x] Using fixture builders or factories for test data: realtime and dataentry harnesses, library story fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `kanban-busy`, `channels` and `kanban-card-counts` specs, 13 tests, locally
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-OCRBAW and the notes table
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint,
arch-lint, plimsoll, comment-lint clean; `go test -race` on the touched
packages; channel conformance with sqlite and postgres tags; markdown corpus
round trip; frontend typecheck, eslint, vitest (5267 tests); library vitest (627
tests), `vue-tsc -b` and Storybook build; markdownlint; `just docs`.

**Deviations:** an e2e spec added (the plan only had a Go integration test);
`RlBusyIndicator.test.ts` lives in the app tests because the library vitest runs
stories only; the busy result is keyed by face address; the security fix uses a
name filter on the waiting query (`one` types read unnamed channels only); busy
field labels are hidden unless `show_label: true`.

## Quality

- [x] Code follows project patterns (check similar code): batch read like the inbox (`ListWaiting`, one anchor batch, one role prime); `omitzero` and `queryGet` like `comment_counts`; reduced motion like the existing `reduce` convention
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one minute clock shared by every indicator;
session state through `currentSession`
- [x] No security issues introduced: rela-security-reviewer minor (row ceiling oracle) fixed with the name filter
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
