---
id: IMPL-IXXXJ2
type: implementation-checklist
title: 'Implementation: Sessions 4/4: non-interrupting timers'
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `realtimecfg/reminders_test.go` (shape, limits, refusals, guard names); `realtime/reminders_test.go` and `timers_reminders_test.go` (slot grid, reload changing every/first, prune, close/reopen/restore, paused reminders, backlog bound, budget); `channels/reminders_test.go`
- [x] Integration tests written (test full flow, not just units): `channelstest` reminder conformance on all four backends including damaged records; sqlite v25 to v26 open test; postgres 0033; dataentry, MCP, OpenAPI, CLI and wire golden tests
- [x] Happy path implemented: `session.reminders` posts the declared message type every `every` while the session is in an `in:` state, counting posts for guards as `session.reminders['name']`
- [x] Edge cases from planning handled: missed slots skipped, reload changing every/first, undeclared records pruned, move wins over reminder, close freezes, reopen/restore resume, `max` reached, broken props pause only that reminder, damaged rows skipped
- [x] Error handling in place (errors surfaced, not swallowed): failed posts and damaged rows log once (rate-limited); refused configs name the reminder and reason

## Test Quality

- [x] Using fixture builders or factories for test data: conformance harness and an injected clock
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: backend feature without new UI; the timer loop tests run the assembled flow)
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-61IBGQ
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build, vet and golangci-lint on all four tags; `go
test -race` on realtime, realtimecfg, channels, dataentry, mcp, cli, openapi,
apiwire, sqlitedb; sqlite- and postgres-tag tests on touched packages (13
dataentry pile goldens fail under postgres identically on the base commit);
arch-lint, plimsoll, comment-lint clean; frontend vitest, vue-tsc, eslint; `just
coverage-check` passes; `just docs` idempotent.

## Quality

- [x] Code follows project patterns (check similar code): 4/3 timer loop and DueWake, store-side CAS under the lock, conformance harness, DamageWarner
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): shared `applySessionStep` in both stores;
one due function for timers and reminders
- [x] No security issues introduced: rela-security-reviewer finding RR-NJB38V addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
