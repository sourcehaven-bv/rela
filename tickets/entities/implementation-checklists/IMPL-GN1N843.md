---
id: IMPL-GN1N843
type: implementation-checklist
title: 'Implementation: archplan weekly: trace batch, reconcile, conflicts and flows'
started: "2026-11-12"
completed: "2026-11-12"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the diagram tests use an inline JSONL trace, like the existing TestRender)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object~~ (N/A: compares function names from a literal trace)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `SEQTRACE_SUITE=routes SEQTRACE_OPEN=0 just seqtrace-demo`: exit 0. The new requests answer as labelled: conflict detail editor 403 / manager 200, resolve manager 200, batch reader 403 / editor 200, reconcile reader 200 / no identity 401, view entry reader 200 / hidden 404, flow tasks and runs 200, flow start reader 403 / manager 201.
- No trace is named after a handler closure any more (only `requestStats.func1`, where no handler ran).
- `archplan ci --json` on the traced model: no new security finding, no worse ratchet metric.
- `go test ./tools/seqtrace/...` passes; `golangci-lint run ./tools/seqtrace/...` reports 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
