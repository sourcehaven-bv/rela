---
id: IMPL-N9Y5SRZ
type: implementation-checklist
title: 'Implementation: Replace neoq with River on postgres and an in-house memory queue'
started: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (memqueue_test.go: panic, key held during retry, payload copy, enqueue before start, Flush duplicate; retry_test.go: retryDelay bounds)
- [x] Integration tests written (jobstest.RunAll on both tiers against a real postgres; neoqcutover_test.go seeds neoq tables in a schema-pinned DSN)
- [x] Happy path implemented
- [x] Edge cases from planning handled (deadline before next retry, close during handler, unknown kind on River, locked neoq rows, NULL current_schema)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: jobs are three-field literals; the suite's NewQueue factory builds each queue)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Memory tier: `go test -race ./internal/jobs/... ./internal/appbuild/` pass;
  goleak shows no goroutine left after Close.
- Postgres tier: `RELA_TEST_DATABASE_URL=... go test -race -tags postgres
  ./internal/jobs/...` pass against local PostgreSQL 18, including the
  schema-pinned DSN, cross-process close, panic, cutover and locked-cutover
  cases.
- The cutover was checked by seeding neoq's tables with a keyed, a retried
  and a processed job: the first two ran under River with their attempt
  count continued, the processed one did not run, and the neoq tables were
  dropped.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (idempotencyID is shared by both queues and the cutover; handlerTimeout moved to jobs.go)
- [x] No security issues introduced (DSN never in an error; payloads never logged; tables stay in the tenant schema)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
