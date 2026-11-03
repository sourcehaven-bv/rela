---
id: IMPL-XLOSLZM
type: implementation-checklist
title: 'Implementation: Compact session header by configuration'
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

- Config (AC 1, 2): `TestChannelsPresentation_Validate`,
`TestValidateChannels_SessionDisplay` (unknown value refused, warning for a type
without a session) and the configedit allowlist cases pass.
- Previous state (AC 4): `TestApplySession_Previous`; conformance
`SessionPrevious` and `SessionMoveFactsKept` pass on memory, file (MemFS and
disk), sqlite and postgres (local PostgreSQL 18,
`RELA_TEST_DATABASE_REQUIRED=1`). Migration rungs: sqlite v32 to v33 and pg 0040
tests pass.
- Wire and API (AC 3, 4): `TestConfig_ServesChannels`, `TestSessionView_Previous`,
wire golden files and wireFields test pass; OpenAPI tests pass.
- SPA (AC 5 to 9): 10 new stories with play tests pass in Chromium;
`channelView.test.ts` and `ChannelsPanel.test.ts` cover the mapping and the
`compact` prop.
- End to end (AC 5, 7, 8): Playwright against a real rela-server, auto and
compact at 60rem and 24rem, including a reload that reads `previous` from the
store. Screenshots taken in that run: compact at 60rem shows "Drafting -> Review
-> Drafting, Approved" and the toggle; unfolded shows the full list on its own
row without the strip; at 24rem the header reads "Session · Review" with the
toggle.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
