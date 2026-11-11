---
id: IMPL-R1UK2FI
type: implementation-checklist
title: 'Implementation: Flows docs and CLI promise retries that do not exist (#2170)'
started: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: text change; no new flow)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: planning lists none)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: extends an existing fake)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the fake returns a fixed id, FLR-1, which the existing assertions use too)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: a failing first step cannot be caused from a valid flows.yaml without a broken store; the unit test checks the message)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: none)

**Verification Evidence:**
`go test ./internal/cli -run TestFlowRun` passes with the new checks
(criterion 1). The vitest suites for `src/components/flows` and `src/i18n`
pass, and `npm run typecheck` is clean. A search of `docs/flows.md`,
`docs/mcp-server.md`, the CLI, the MCP tool and the web app strings for
"will be retried", "is retried", "next attempt", "tries again later" and
"retried with backoff" finds only the new manual-retry sentence
(criterion 2).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
