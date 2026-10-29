---
id: IMPL-5GAR9H
type: implementation-checklist
title: 'Implementation: Authored one-click presets for session transition entries'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg` `TestButtons_Load`, `TestButtons_Problems`, `TestButtons_PropsProblems`, `TestButtons_Guards`, `TestButtons_DisjointRoleRecorded`, `TestButtons_Offers`, `TestButtons_StateChecks`; `realtime` `TestAffordances_Entries`, `TestSessionView_Button`, `TestPost_PropsCapMatchesStore`; SPA `decisionViews presets`, `useSessionDecisions presets`
- [x] Integration tests written (test full flow, not just units): `TestButtons_DeciderAgrees` (real posts for single, two-role and disjoint-role callers), `TestPost_ButtonPreset`, `TestWireChannelSession_Button`, `TestChannelWireGolden`, MCP `TestReadSession`, `ChannelsPanel preset buttons`
- [x] Happy path implemented: `on.<type>.button: {label, props}` in realtime.yaml; `entry` and `button` on transition rows; per-caller `_actions.entries`; one-click preset buttons in the SPA through the normal post path
- [x] Edge cases from planning handled: earlier entries that take the click or cannot be judged at load hide the button for any caller who admits them (two-role and disjoint roles); person classes only; duplicate labels per caller; integers beyond 2^53 refused; ref and markdown presets refused; confirm posts the confirmed view; missing `entries` falls back to the old button
- [x] Error handling in place (errors surfaced, not swallowed): every refused preset names its path and reason at load; a guard error at load refuses or hides the button; prop rules return sentinel errors mapped to the existing denials

## Test Quality

- [x] Using fixture builders or factories for test data: realtime session fixtures, channel wire fixtures, SPA panel harness
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no e2e in the plan; the decider agreement test posts for real)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-LI29KJ and the notes table
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint,
arch-lint, plimsoll, comment-lint clean; `go test -race` on realtimecfg,
realtime, dataentry, mcp, openapi, apiwire/v1 and cli; channels conformance on
sqlite and postgres; markdown corpus round trip; frontend typecheck, eslint,
vitest (5345 tests); markdownlint; `just docs`; archplan ci and coverage check.

**Deviations:** a button records every earlier entry that could take its click,
also for disjoint roles; the guard binding is shared as
`realtimecfg.GuardHolds`; `call.fireable` removed to stay under the plimsoll
method cap; test names adjusted where the planned files did not exist.

## Quality

- [x] Code follows project patterns (check similar code): button checks beside the branch checks in realtimecfg; affordances next to `_actions.transitions`; SPA views in `presetViews.ts` beside `channelView.ts`
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): prop rules once in
`MessageType.NormalizeProps`; guard binding once in `GuardHolds`; one `Offers`
rule for load and affordance; trigger literals tracked in TKT-852C0M
- [x] No security issues introduced: rela-security-reviewer found one minor (integers beyond 2^53), fixed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
