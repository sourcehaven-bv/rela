---
id: IMPL-5ZZY9UU
type: implementation-checklist
title: 'Implementation: Flows M5e: dogfood the tickets checklist workflow in tickets/flows.yaml'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestRelatedReadsOutgoingTargets`, `TestTicketsTriggersCovered`)
- [x] Integration tests written (test full flow, not just units): `tickets/tests/checklist-workflow.test.lua`, 18 cases on an in-memory copy of the tickets config
- [x] Happy path implemented
- [x] Edge cases from planning handled (re-entry, started kept, ready/blocked/wont-fix, create-in-status, gate skipped, no docs checklist)
- [x] Error handling in place (errors surfaced, not swallowed): `related` raises on an undeclared relation type and on read errors; a hidden target is skipped like `list` does

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the Lua suite uses small `ticket`/`bug` helpers over `actor:create`)
- [x] No hardcoded values in assertions when object is in scope (ids come from created entities; dates from `t.today()`)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `rela --project=tickets test --require-tests`: 18 passed.
- Mutation: removing `ticket-review-checklist` from a scratch copy of
`tickets/flows.yaml` failed 2 cases (its own and the re-entry case).
- Renaming the `work-completed` case prefix failed `TestTicketsTriggersCovered`
with both messages (missing case, stale prefix); restored.
- `go test ./internal/apptest/...`, golangci-lint, plimsoll, comment-lint and
arch-lint clean. `fileRun` has 40 methods, at the plimsoll cap.
- The gate case found that relation-count rules give no write warnings at
all; the case asserts only that the write is accepted.

## Quality

- [x] Code follows project patterns (check similar code): `related` mirrors `list` (gated reader, ordered by id)
- [x] Checked for DRY opportunities: the table building of `related` and `list` is three lines each; kept apart
- [x] No security issues introduced: reads go through the actor's gated reader; CI step uses no event inputs
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
