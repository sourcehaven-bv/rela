---
id: IMPL-JXOY9O
type: implementation-checklist
title: 'Implementation: Extract the webhook find/create/then mapping core out of dataentry'
started: "2026-10-14"
completed: "2026-10-14"
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
- AC1: `go test ./internal/dataentry/ ./internal/dataentryconfig/` passes. The six postgres tests (`-tags postgres -run WebhookConflict`, against `rela_test`) pass, including cross-process appends. The only test-file edit is one comment naming `ingest.FlattenToLine`.
- AC2/AC3: `internal/ingest` has its own tests with fake reader, writer and store: workflows, lowest-id winner, match-value seeding, CAS version on append, retry on unique and version conflicts, exhausted budget, non-conflict not retried, dependency checks, cancelled context, faces. Coverage 94.8%.
- AC4: `just arch-lint` reports no warnings.
- Manual: a `rela-server` on a fresh `rela init` project with an `alert` webhook. The first POST answered `created`. The second POST with the same host answered `updated` on the same id, appending to `## Log` and setting status. A payload `msg` carrying `\n## Forged` was stored as one line. A POST with no host created a new entity rather than matching one with an empty title.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
