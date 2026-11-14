---
id: IMPL-UQB2QFX
type: implementation-checklist
title: 'Implementation: Rejected channel batches are not charged against the rate limit'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestPostBatch_Charges)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the fix is inside realtime.Service.PostBatch, which every transport calls; the HTTP, MCP and CLI suites still pass)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a batch refused before the charge stays free; an over-rate batch stays a 400)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing harness and slow helper)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values in the new assertions)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: the test compares status codes and denial types only)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: a one-line reorder in the service; covered by the service test and the transport suites)
- [x] Each acceptance criterion verified with test scenario from planning (invalid batch charged, 429 once spent, early refusal free)
- [x] ~~Edge cases manually verified~~ (N/A: covered by TestPostBatch_Charges and TestPostBatch_Refusals)

**Verification Evidence:**
TestPostBatch_Charges fails on develop (second invalid batch returns 400, not
429) and passes with the fix. `go test` passes for internal/realtime,
internal/mcp, internal/cli and internal/dataentry.

## Quality

- [x] Code follows project patterns (check similar code) (same charge point as Post and Edit)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (closes an availability gap; no ACL or data path touched)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
