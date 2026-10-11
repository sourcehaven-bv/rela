---
id: BUGA-V9ZQZY
type: bug-analysis-checklist
title: 'Analysis: SQLite force-live purge was re-captured by the sweep'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

## Notes

Reproduced by restoring the old column order in `liveEntityHash`: the new
conformance case `SweepBacklog/ForceLivePurgeIsNotRecaptured` fails with two
versions (the tombstone and a re-captured `update`). Conditions: SQLite build,
`history-purge --all --force-live`, then one sweep tick.

Fix: the column order was already corrected on develop by #1811; this change
makes the query use `getEntitySQL` so it cannot drift from `scanEntity` again.
Related areas: the other `scanEntity` callers use `entityColumns`; `search.go`
scans `id, face, type` with its own scanner, consistently. pgstore was checked
by the issue author and is not affected; the conformance case runs on postgres
in CI.
