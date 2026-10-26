---
id: IMPL-2LUE2G
type: implementation-checklist
title: 'Implementation: Sessions 4/5a: message-driven graph outputs'
started: "2026-10-26"
completed: "2026-10-26"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg/outputs_test.go`, `template_test.go` (shape, limits, refusals incl. timer/trigger transitions naming TKT-RAN1YA); `realtime` `TestOutputs_*`, `TestRunner_*`, `TestOutputCheck_*`, supersede, lock and detached-context tests; `channels/outputs_test.go`
- [x] Integration tests written (test full flow, not just units): `channelstest` output conformance on file, sqlite and postgres; `TestOutputWriter_*` against the real entitymanager (set, create, field_denied, unmatched principal, retry as poster, removed user); `TestWorker_OutputsRoundBesideTheLoop`; dataentry, MCP, OpenAPI, CLI and wire golden tests
- [x] Happy path implemented: a message that moves a session runs the transition's `set:` and `create:` outputs as the poster through the gated entitymanager; `on_final: close` closes the channel
- [x] Edge cases from planning handled: pre-check 403 before the post, field-gated sets, crash windows per tier (started/unconfirmed), exactly-once create when stores join, per-property supersede, claim renewal, moderator release and stale sweep exclude marks, transition removed by reload, retry window and removed users
- [x] Error handling in place (errors surfaced, not swallowed): failed outputs recorded on the notice with a code; bind errors are 500s; new problem types `session_output_denied` (403) and `outputs_pending` (409)

## Test Quality

- [x] Using fixture builders or factories for test data: conformance harness, fake writer via `appbuildtest.WithChannelOutputs`, injected clock
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no UI in this slice; the writer tests run the real entitymanager end to end)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-8L0IW8
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on default,
memorybackend, postgres, sqlite and maildemo,mailmanual; `go test -race` on
touched packages; sqlite- and postgres-tag tests (pile goldens in
TestGoldenParity fail identically on develop; TestLeaseCLI needs
RELA_DATABASE_URL); arch-lint, plimsoll, comment-lint clean; frontend vitest
4717, vue-tsc, eslint; `just coverage-check` passes; `just docs` idempotent.

## Quality

- [x] Code follows project patterns (check similar code): `promote_to` create checks, PatchEntity for partial writes, conformance harness, leader worker
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): shared create checks with `promote_to`; one
template engine for set and create
- [x] No security issues introduced: rela-security-reviewer findings RR-YXUZ19 and the two others addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
