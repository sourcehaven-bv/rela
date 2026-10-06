---
id: IMPL-QK6RNK
type: implementation-checklist
title: 'Implementation: pgstore change-feed listener fails when RELA_DATABASE_URL sets pool_max_conns'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestCrossProcessPropagation_PoolTunedDSN`)
- [x] Integration tests written (test full flow, not just units): the regression test opens two full stores and requires a live NOTIFY delivery, with catch-up disabled
- [x] Happy path implemented
- [x] Edge cases from planning handled (URL and key/value DSNs; reconnect path reuses the parsed config)
- [x] Error handling in place (errors surfaced, not swallowed): a parse error is returned from `startListener` and logged by `Open` as before

## Test Quality

- [x] Using fixture builders or factories for test data (`freshFeedSchema`, `openWriterDSN`)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `TestCrossProcessPropagation_PoolTunedDSN` failed before the fix ("unrecognized configuration parameter pool_max_conns", no event in 5 s) and passes after. Full pgstore suite passes with `-race`.
- `rela-server-postgres` against the perf project with `pool_max_conns=2`: the old build logs "cross-process change feed unavailable"; the fixed build logs no such warning.
- `docscapture` tests with `RELA_DATABASE_URL=...&pool_max_conns=4`: the old code fails in `standUp` with the same FATAL; the fixed code passes the package.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (two call sites, each a two-line parse; not extracted)
- [x] No security issues introduced (no DSN content is logged; pgx redacts passwords in parse errors)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
