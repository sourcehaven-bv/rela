---
id: IMPL-A6A4K7T
type: implementation-checklist
title: 'Implementation: Show each session decision button once'
started: "2026-11-04"
completed: "2026-11-04"
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

- Unit: `channelView.test.ts` (placeDecisions, buttonTypesOf, rowDecisions
keeps answer presets), `useSessionDecisions.test.ts` (plain form decision is
compose mode, refusal records the click generation, Other then Cancel ends the
reply, buttonTypes held across the move gap), `ChannelsPanel.test.ts` (AC 1, 2
table, 3, 4, 7 focus cases), `OpenChannel.start.test.ts` (two channels, focus
stays per channel), `InboxWaitingItem.test.ts` (cause 0 places below the row,
presets on the question). Full suite: 387 files, 5849 tests pass.
- Stories (Playwright, chromium): `RlChannelComposer` DecisionMode,
DecisionModeWithPicker, AllButtonTypes, ButtonTypesSelect; `RlChannel`
DecisionsOnceWithJump, ReplyFollowsPicker, ReplyWhenAllButtonTypes;
`RlChannelSession` WithGoToDecision, CompactWithGoToDecision,
NarrowWithGoToDecision; `RlSegmentedControl` NoneChecked. Library suite: 133
files, 682 tests pass.
- e2e against a built server: `channels.spec.ts` and `kanban-busy.spec.ts`,
16 tests pass. They click the header and row buttons, check Go to decision moves
the focus to Approve on the message, and check the compact header shows Go to
decision.
- No separate manual click-through beyond the e2e runs.

## Deviations from the plan

- Focus follow: a move from the header to the row goes to Go to decision
even when the same key exists on the row (the plan listed "same key" first).
Reason: the plan's own note says a header focus must go to the jump so the page
does not scroll to the row.
- Focus follow: when Go to decision had the focus and the decisions move
to the header, the focus goes to the first header decision. Reason: the test
plan (AC 7) asks for "focus on header Approve"; the rule's fallback (the header)
would not meet it.
- Inbox: the row places the decisions on the message only when its seq is
the session cause, and `cause` 0 counts as no cause. Reason: with cause 0 the
old and the planned code both put them on a message that is not the cause; the
plan's pinning test failed until this guard.
- `RlChannelComposer` keeps the first pickable type as the fallback of
`current` (not `defaultType`). Reason: keeps the `select` event at mount, which
the SelectEmits story relies on.
- The composer's Cancel decision now focuses the first field after the app
clears `compose` (plan AC 7 "After Cancel decision, focus goes to the composer's
first field"); done in the composer, not in the app.
- `docs/realtime.md` and `docs/data-entry.md` are generated; the edits are
in `docs-project/entities/guides/GUIDE-realtime.md` and `GUIDE-data-entry.md`,
then `just docs`.
- `ChannelsPanel` test "a header decision returns the focus to the header"
replaces the old header assertion in "posts approve from the header".

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
