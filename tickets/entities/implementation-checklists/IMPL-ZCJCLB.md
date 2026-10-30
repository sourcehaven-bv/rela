---
id: IMPL-ZCJCLB
type: implementation-checklist
title: 'Implementation: Session default_in: offer keyed types only where they apply'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg` `TestParse_SessionDefaultIn`, `TestDefaultIn_Derive`, `TestDefaultIn_AbsentKeepsToday`, `TestDefaultIn_Checks`, `TestDefaultIn_DerivedSkipsChecksThatCannotFire`, `TestCheckReload_DefaultIn`, `TestDocsExample_DefaultIn`
- [x] Integration tests written (test full flow, not just units): `TestDefaultIn_PoCEquivalent`; `realtime` `TestAffordances_DefaultIn` over the shared `messageInRows`, `TestPost_DefaultIn`, `TestSessionTriggerPost_DefaultIn`, `TestReload_DefaultIn`; `dataentry` `TestChannelsHandler_SessionWrongState` derived case
- [x] Happy path implemented: `session.default_in: keyed` gives a type with no `in:` the states whose `on:` keys it, in `session.states` order; 409 detail and trigger warning name `session.default_in`
- [x] Edge cases from planning handled: every entry form keys its state (branch lists, guarded, shadowed, role-limited, `by: trigger`); an authored `in:` wins and keeps its checks; unkeyed, reminder and unkeyed `choices` types stay free; reload on and off; the two skipped checks shown unable to fire
- [x] Error handling in place (errors surfaced, not swallowed): bad values refused with their path and a hint under a state; refusal of an authored `in:` suggests dropping it; a warning when the key changes nothing

## Test Quality

- [x] Using fixture builders or factories for test data: `pocRealtime`, `pocRealtimeDefaultIn`, `messageInRows(fixture)`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: config-only change; the PoC equivalence and reload tests run the real service)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-3JOPEX and the notes table
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint,
arch-lint, plimsoll, comment-lint clean; `go test -race` on realtimecfg,
realtime, openapi and dataentry; texthazard and markdown corpus tests;
markdownlint; `just docs`; archplan ci and coverage check.

**Deviations:** `checkDefaultIn` split out for `funlen`; reload rows in a new
test because the shared table always reloads one file; some planned test cases
adjusted where the loader refuses them.

## Quality

- [x] Code follows project patterns (check similar code): `default_<key>` naming like `default_view` and `default_sort`; closed enum checked like `on_final`; derivation beside the existing `in:` compile
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one `InKey` helper for both rule strings;
affordance rows shared by both fixtures
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
