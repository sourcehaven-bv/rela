---
id: IMPL-GLG831U
type: implementation-checklist
title: 'Implementation: Group quiet channel messages across session notices'
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

- `channelFold.test.ts` (44 tests): joins between members, waiting,
final and unknown notices, leading and trailing notices, hidden members,
hidden-only runs, removed plus notice, kept notice, `showAll`, no quiet types,
expanded order, labels and counts, append and prepend.
- `channelView.test.ts`: `toStatus` from the session state, and none for
an unknown state or no session.
- `useSessionDecisions.test.ts`: a working state with a quiet cause puts
the decisions in the header; a waiting state or a shown cause keeps them on the
cause.
- `ChannelsPanel.test.ts`, "quiet types" (9 new): one fold across
notices with its aria, decisions in a waiting state, the owner decision in the
header in a working state, notices always announced and once only, focus to the
fold, focus kept in an expanded fold, focus after Load older, a reply quote to a
folded notice.
- Stories: `RlChannelFold` `WithStateChanges` and
`DutchWithStateChange`; `RlChannel` `QuietTypesAcrossNotices` and
`QuietTypesNoticesRefold` (scroll anchor within 2px when a reload turns notices
into working ones).
- E2E `channels.spec.ts` "quiet messages across session notices": real
server, one fold "5 messages, 2 state changes", the move into Plan review stays
in view with its decision, the move back after the owner's decision stays in
view, and new progress folds after it. All 17 channel and kanban-busy specs
pass.
- Gates: app and library vue-tsc, lint (no new warnings), 5894 unit
tests, 690 library story tests, library `npm run check`, e2e lint and typecheck,
`just docs`, lint-md.

**Deviations:**

- The descriptor re-fold and scroll-anchor checks (AC 7a) are a browser
story, `QuietTypesNoticesRefold`, and not a ChannelsPanel test. jsdom has no
layout, so it cannot measure the anchor offset.
- The row-focus hand-off skips decision buttons. The decisions already
hand their focus on to the session header (TKT-A0RTC52); without the skip, the
new hand-off took that focus first. The existing test "from the row to the same
decision in the header when the cause is removed" caught it.
- The e2e room uses a waiting state for the feature's `owner`, not for
`lead`: `waiting_for` needs a role with a known holder.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
