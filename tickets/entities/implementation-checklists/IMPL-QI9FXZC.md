---
id: IMPL-QI9FXZC
type: implementation-checklist
title: 'Implementation: Flows M2: step forms and run UI'
started: "2026-11-10"
completed: "2026-11-10"
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

- Go: `TestFlowControl_CompleteStep`, `_RunViewAndActions`, `_Tasks`,
`_RunList`, `_TasksBudget` (appbuild); `TestFlowsAPI_*` (dataentry);
`TestFlowTools_*` (mcp); `TestCurrentFlows_ResolvesPerCall` (mcpwire);
`TestRemoteMCPDeps_Flows`; `TestEngine_ReleaseClaim`; `TestHolderLookup_Headers`
(order and limit); `TestWithFlowRuns_NoRelations`. All pass with `-race`.
- SPA: vitest for the flows API client, `useFlowList`, `flowView` and the
flow components; typecheck and eslint clean.
- E2E `e2e/tests/flows.spec.ts` drives the real server: start a run from the
entity page, complete the step through the form, see the run finish, and the
tasks and runs lists. It passed 5 out of 5 after the step-form load guard fix.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
