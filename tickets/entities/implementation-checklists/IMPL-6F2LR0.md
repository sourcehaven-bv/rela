---
id: IMPL-6F2LR0
type: implementation-checklist
title: 'Implementation: Guarded branching transitions per message type'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestSession_BranchList`, `TestParse_BranchListKeys`, `TestSession_BranchListProblems`, `TestSession_BranchListWarnings`, `TestTransition_Admits`, `TestResolveMark_Branch`, `TestOutputs_BranchUsesFiredEntry`, `TestOutputs_SharedTargetTriggerCloseMark`; SPA `channelView.test.ts` composer grouping and guard cases, `useSessionDecisions.test.ts` form-decision suite
- [x] Integration tests written (test full flow, not just units): `TestSessionBranches_FirstWins`, `_PosterRulePerEntry`, `_NoMatchStoresWithoutMove`, `_GuardErrorStops`, `_GateRefusesOtherEntry`, `_GateThroughPost`, `_ConcurrentDecisions`; `TestSessionView_BranchEdges`, `TestInbox_BranchTypeOnce`, `TestWireChannelSession_BranchEdges`; ChannelsPanel hint and form-decision wiring; e2e `channels.spec.ts`
- [x] Happy path implemented: `on.<type>` takes a list of up to 8 guarded entries, first match wins, per-entry poster rule and outputs; read views list every edge; composer names every target; form decisions post with `expect_generation`
- [x] Edge cases from planning handled: no entry applies (stored, no move), guard error (fail closed), different entry under the lock (no move), shared targets without outputs, shadowed entries per role, single-entry form unchanged
- [x] Error handling in place (errors surfaced, not swallowed): load problems keyed `on.<type>[i]`; guard failure logged with the entry key; no-catch-all warning

## Test Quality

- [x] Using fixture builders or factories for test data: realtime harness and config fixtures, library story fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `channels.spec.ts` 9 of 9 locally
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-N0AM2O
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint 0
issues; arch-lint, plimsoll, comment-lint clean; `go test -race` on realtime,
realtimecfg, realtimelimits, mcp, openapi, dataentry; channel conformance with
sqlite and postgres tags; frontend typecheck, eslint, vitest (5203 tests);
library vitest (613 tests) and Storybook build; markdownlint; `just docs`;
archplan ci reports only the five known sqlite `entity_search_*` tables.

**Deviations:** a guard that fails to evaluate stops the decision (security
review, reverses plan item 5); denial titles keep the `...outputs[i]` format;
the composer emits `select` so a type switch drops the form decision; `mayMove`
hint when only some entries are guarded; one poster rule `Transition.Admits`
shared by decider and load checks.

## Quality

- [x] Code follows project patterns (check similar code): guards through `internal/predicate`; gate under the store lock via `SessionStep`; load problems keyed like the existing config keys
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one shadowing check (`firesFor`), one
poster rule (`Transition.Admits`)
- [x] No security issues introduced: rela-security-reviewer minor finding (guard error fall-through) fixed by failing closed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
