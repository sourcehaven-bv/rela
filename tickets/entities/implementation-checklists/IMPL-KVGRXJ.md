---
id: IMPL-KVGRXJ
type: implementation-checklist
title: 'Implementation: Sessions 4/6: input_required and handoff'
started: "2026-10-26"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg` status/waiting_for load rules, can-answer, deadline-reset chains, answer-grant warning; `realtime` waiting view, waiting_on_me, list budget, stranded sessions, handoff re-read; CLI waiting line; stepper story play tests
- [x] Integration tests written (test full flow, not just units): `TestHandoff*` in `internal/appbuild/realtime_handoff_test.go` against the real entitymanager (claim, handoff, field-gate pre-check 403); MCP `read_session` with watch in `internal/mcp/tools_session_test.go`; `TestDocsExample_Realtime`
- [x] Happy path implemented: per-state `status` and `waiting_for`, per-reader `waiting_on_me`, handoff and claim through `set:` on the role property, MCP `read_session`, stepper status badges
- [x] Edge cases from planning handled: final states, input_required iff waiting_for, unbounded roles refused, stored "" as initial, stable unstored `entered_at`, stranded as failed, redacted holders, failed re-read, deadline-reset chains, field-level refusal before the post
- [x] Error handling in place (errors surfaced, not swallowed): load problems name the state and role; `no_session` and `session_output_denied` errors

## Test Quality

- [x] Using fixture builders or factories for test data: realtime fixtures, testdata policy, injected clock
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the stepper is covered by story play tests and the flow by appbuild tests)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-97H3XU
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on all five tags (0
issues); `go test -race` on touched packages and async tests at -count=5;
sqlite- and postgres-tag tests (only the pre-existing TestGoldenParity pile
goldens fail); arch-lint, plimsoll, comment-lint clean; markdownlint clean;
frontend vitest, vue-tsc, eslint; `just coverage-check` 85.1%; `just docs`
idempotent.

## Quality

- [x] Code follows project patterns (check similar code): 5a outputs pre-check, realtimecfg load checks, collection-read budget tests
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): `roleAdmits` shared by `call.can` and
`waitingOf`
- [x] No security issues introduced: rela-security-reviewer finding (1 minor) addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
