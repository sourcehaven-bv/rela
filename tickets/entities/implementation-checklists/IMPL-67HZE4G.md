---
id: IMPL-67HZE4G
type: implementation-checklist
title: 'Implementation: Declarative LLM actions: llm: block on data-entry actions'
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

Unit: `internal/llmtask` (schema, request, reply parse: 94.7% coverage),
`internal/dataentryconfig` (`llmaction_test.go`), `internal/ai` (response
format), `internal/conditionlint`, `internal/mcp`
(`TestActionResultView_Proposal`), frontend `llmProposal.test.ts` and
`LLMProposalDialog.test.ts` (4 tests). Integration:
`internal/dataentry/actions_llm_test.go` drives the real HTTP route with a fake
provider through `Seams.AI` and a real ACL policy.

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

Scratch project (`.ignored/llm-demo`: ticket, concept, `affects`) served by
`rela-server` with the built SPA, against Ollama `gemma4:12b`.

- AC1: `POST /api/v1/_action/triage` returned a proposal (priority low to
high; affects Authentication and Billing). `rela show` afterwards: priority
still low, no relations.
- AC4: in the browser, the detail button showed "Bezig…" while the request
ran, then the dialog listed the changes. Billing unticked, priority changed to
medium from the CLI before clicking apply. Result: priority stayed medium and
the dialog named it as changed since the proposal; only Authentication was
linked.
- Found and fixed: after a partial apply the dialog left applied items
ticked, so a second apply resent them. Applied and stale items are now unchecked
and locked (test "applying again retries only what did not go through").
- Timeout: a second run took longer than 2 minutes. The response was 502
`ai_failed` "The AI provider did not answer in time"; the provider error went
only to the server log.
- AC2, AC3, AC5, AC6: covered by `TestLLMAction_HiddenFieldsAreNotSent`,
`TestLLMAction_Refusals`, llmtask parse tests, config validation tests and
`TestActionResultView_Proposal`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Checks: `just arch-lint`, `golangci-lint` on the changed packages, comment-lint
gate, plimsoll, archguard (peer reads now use gated `GetAddress`, bounded by the
input limit of at most 50), frontend typecheck, ESLint and unit tests.
