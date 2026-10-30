---
id: IMPL-8DZLLY
type: implementation-checklist
title: 'Implementation: Busy board live refresh on session moves'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestLocal_MovedKeys`, `TestSubscribeLive`, `TestTakeLive_OneCriticalSection`, `TestPostMoved`, `channelstest` `AppendReportsCreatedChannel`, `TestRecordBusy`, `TestRoundFilter_SelectsBusy`, `TestLiveHub_CollectRechecksBusy`
- [x] Integration tests written (test full flow, not just units): `TestLive_BusyListWakesOnSessionMove`, `TestLive_BusyListIgnoresMessageOnly`, `TestLive_BusyHiddenAnchorNoWake`, `TestLive_BusyWakePerPrincipal`, `TestLive_BusyOverflowNoFallbackFrame`, `TestLive_BusyListRecheckWhileSyncing`, `TestLive_BusyWakeCoalesces`, `TestLive_BusySafetyReplay`, `TestLive_BusyMovedKeyAloneStartsRound`, `TestLive_BusyWakeSetFromReplay` "busy read fails"
- [x] Happy path implemented: session moves of the busy channel type wake busy list subscriptions whose gated page holds the anchor; message-only writes do not
- [x] Edge cases from planning handled: channel creation counts as a move (`CreatedChannel`); no fallback frame for busy lists in topic rounds; recheck while syncing; busy boards join the safety replay for moves from other processes; all change sets taken under one lock; a failed busy read keeps the wake
- [x] Error handling in place (errors surfaced, not swallowed): a busy read error keeps the wake from the gated rows; a non-200 replay clears it

## Test Quality

- [x] Using fixture builders or factories for test data: manual-ticker busy live env, `job` and `task` fixture types
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the live tests drive real subscriptions over the router; no e2e in the plan)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-C9AHJ7 and the notes table; mutation checks confirm each rule is pinned
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint,
arch-lint, plimsoll, comment-lint clean; `go test -race` on dataentry, realtime,
channels (all four stores), dataentrywire and mcp; new live tests with `-race
-count=5`; texthazard and markdown corpus tests; markdownlint; `just docs`;
archplan ci and coverage check.

**Deviations:** AC 4 drops `retention` from the `job` fixture's delete rule,
since under retention only a closed (not busy) channel can be deleted; AC 2
revokes read through group membership because the policy is fixed at router
build; AC 8 uses the round budget instead of the cap for a stable cut; graph
write subtests step three ticks; follow-ups TKT-IV6XUP and TKT-S3YYGM.

## Quality

- [x] Code follows project patterns (check similar code): busy wake beside the inbox wake in the live hub; moved keys beside the durable set in `realtime.Local`; no-fallback treatment like the inbox
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `postMoved` helper; one `TakeLive` for
every change set; shared safety clock
- [x] No security issues introduced: rela-security-reviewer found two minor timing residuals; one fixed (pending-only pairs get no frame in topic rounds), one documented in the code
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
