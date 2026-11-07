---
id: IMPL-6D1ZZOW
type: implementation-checklist
title: 'Implementation: neoq: stop the cron goroutine on queue shutdown before tenant eviction'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths in rela; the fork now returns redis start errors instead of calling log.Fatal)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no entity test data)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Old pin (05694be): `TestMemoryQueue_CloseReleasesGoroutines` fails, 23 goroutines vs. baseline 3 after 20 cycles. The postgres variant passes, because the postgres backend already stopped its cron.
- New pin (7cfa988): `go test -race ./internal/jobs/...` ok; `go test -tags postgres -race ./internal/jobs/...` ok against local postgres, including `TestPostgresQueue_CloseReleasesGoroutines`, `TestPostgresQueue_SchemaPinnedDSN` and `TestPostgresQueue_SurvivesAnotherProcessClosing`.
- `go build ./...` with and without `-tags postgres`; golangci-lint on internal/jobs: 0 issues; `just arch-lint` and `just comment-lint` clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
