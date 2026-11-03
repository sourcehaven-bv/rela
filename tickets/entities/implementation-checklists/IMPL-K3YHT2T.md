---
id: IMPL-K3YHT2T
type: implementation-checklist
title: 'Implementation: Place a channel type above the relations by configuration'
started: "2026-11-03"
completed: "2026-11-03"
status: done
---

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

- AC 1: `TestSectionEntry_MarksEntrySourcedSections` (entry flag on the
wire); `channelPlacement.test.ts` (insert index for generated view, custom view
starting with a relation, entry only, no sections);
`EntityDetail.channelPlacement.test.ts` (block order in the page); e2e `channel
placement` in `channels.spec.ts` runs rela-server with `channels.room.placement:
above_relations` and checks in Chromium that the channel's top lies between the
first and last view section, and that posting still works there.
- AC 2: existing `ChannelsPanel.test.ts`, `ChannelsSection.deeplink.test.ts`
and the 14 existing channel e2e tests pass unchanged; the placement test
"renders one block at the end, headed Channels, without placements".
- AC 3: `TestChannelsPresentation_Validate` (tab, sidebar, wrong case,
misspelled key, docs example); configedit allowlist test.
- AC 4: `ChannelsSection narrowed to some channel types` cases in
`ChannelsSection.deeplink.test.ts` (a foreign type's link is ignored, no switch
to the all filter, no scroll).
- AC 5: `holds at most two live keys per block` (four keys for two blocks
in single mode).
- Edge cases: two types in one block, every type placed above, unknown
value from a newer server, distinct heading ids.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
