---
id: IMPL-JRVKBW
type: implementation-checklist
title: 'Implementation: Realtime 3/2: channel store and backends'
started: "2026-10-12"
status: done
completed: "2026-10-12"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no HTTP or service consumer until T3; the store is exercised end to end by channelstest on all four backends)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases pinned by conformance subtests and backend damage tests)

**Verification Evidence:**
Race tests pass on all four backends; postgres ran against a local database through a schema-pinned DSN.
- AC1: channelstest.RunAll (31 subtests) on filechannels (disk and MemFS), sqlitechannels and pgchannels.
- AC2: GaplessSeqUnderConcurrency; SecondInstance with separate semaphores so only the file lock separates processes.
- AC3: Paging, PageLimitIsClamped, AfterRevSeesEdits, AfterRevPagesInRevOrder.
- AC4: Edit, RemoveErasesContent, EditKeepsNoHistory, TestPatchCompaction.
- AC5: Rekey, RekeyConflict, TestChannelAliases_Rename, TestChannelAliases_RenameRaces.
- AC6: DeleteKeepsAudit, TombstoneEntity, TombstoneFace.
- AC7: pgchannels TestSchemaIsolation; sqlite uses the store handle.
- AC8: just arch-lint.
- AC9: Caps, TestPrepareMessage.
- Design review: Incarnation, AnchorModes, CreateManyCap, LongestAnchor, TestAliasHook_SurvivesCancelledRequest, TestLastMessageReadsFromTheEnd, TestTornTail.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
