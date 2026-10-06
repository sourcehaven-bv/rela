---
id: BUGA-046YHE
type: bug-analysis-checklist
title: 'Analysis: pgstore change-feed listener fails when RELA_DATABASE_URL sets pool_max_conns'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`TestCrossProcessPropagation_PoolTunedDSN` fails: FATAL unrecognized configuration parameter "pool_max_conns"; no event within 5 s)
- [x] Minimal reproduction steps documented (bug body; also seen in BUG-9TGOH1 manual runs)
- [x] Environment/conditions noted (postgres build, any DSN with a `pool_*` key, URL or key/value form)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined: parse the DSN once with `pgxpool.ParseConfig` in `startListener` and connect and reconnect with `pgx.ConnectConfig` on a copy of its `ConnConfig`. Apply the same to the `docscapture` admin connection.
- [x] Regression test planned: `TestCrossProcessPropagation_PoolTunedDSN` (live feed only, catch-up disabled).
- [x] Related areas checked for similar issues: `docscapture/scratch_postgres.go` admin `pgx.Connect` has the same defect. `tenant.dsnForSchema` and `docscapture.dsnWithSearchPath` re-serialize the DSN and drop `pool_*` keys, so a tenant pool ignores the operator's pool size; not a connection failure, noted for a separate ticket. Pools built with `pgxpool` are unaffected.
