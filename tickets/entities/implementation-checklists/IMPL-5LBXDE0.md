---
id: IMPL-5LBXDE0
type: implementation-checklist
title: 'Implementation: LLM actions on collection screens (kanban, lists, piles)'
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Backend: `llmtask/batch.go` (BatchRequest, BatchSchema with shared `$defs`,
ParseBatch with per-row failures), `dataentry/actions_llm_batch.go` (payload
checks, batched row gate, one provider call), list/kanban `_actions` keys in
`toolbaractions.go`, config `batch:` and kanban `actions:` validation. Frontend:
`useBatchAction`, list batch branch in `useListActions`, kanban header buttons,
`LLMBatchProposalDialog`.

Tests: `llmtask/batch_test.go`, `dataentryconfig/llmbatch_test.go`,
`dataentry/actions_llm_batch_test.go` (gating, hidden fields, refusals, read
budget at 10 and 50 rows, affordance/gate agreement), and frontend tests for the
dialog, `useBatchAction` and the list branch.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: needs a live AI provider; the HTTP flow is covered end to end in `actions_llm_batch_test.go` with a fake provider)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by the refusal and gating tests)

**Verification Evidence:** Go tests for llmtask, dataentryconfig, dataentry,
script, apiwire and mcp; `vue-tsc -b` clean; 114 frontend test files (1612
tests) pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
