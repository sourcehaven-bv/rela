---
id: IMPL-SERO9P8
type: implementation-checklist
title: 'Implementation: Weekly postgres scale run'
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

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests use a small YAML config literal and an httptest fake server, no entity fixtures)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `scalerun run -scales 1,5` against Postgres.app (database rela_scalerun_local): scale 1 seeded 19,117 entities and 47,549 relations, scale 5 95,577 and 237,455; all 34 endpoint/principal pairs measured with the expected status; RSS peak 94 MiB (scale 1) and 259 MiB (scale 5). `scalerun check` passes on the results with the calibrated limits.
- Benchmarks with -count 6 (about 11 minutes, search reduced from 10k to 5k afterwards); `scalerun check` with two benchmark files produced the benchstat table in the report.
- `just scale-run 1` end to end (builds, run, benchmarks, check, baseline copy).
- Workflow: actionlint clean. It was not run on GitHub.
- Unit tests: `go test ./tools/scalerun/...` (config validation, percentiles, Server-Timing, driver against a fake server, benchstat CSV, Check, report, compareBench notes, parseScales, withSearchPath).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
