---
id: TKT-PU3HUR
type: ticket
title: 'Perf: load-test the version sweep at 1M entities × 10k versions (relations candidate scan)'
kind: test
priority: medium
status: ready
effort: s
---

## Result: DONE — sweep is index-bound and flat with scale. No code change needed.

Load-tested at 10k / 100k / **1,000,000** entities (3× relations via fanout=3),
10,000 settled versions, Batch=500. Harness:
`internal/store/pgstore/sweep_load_test.go` (DB-gated, env-tunable). Full
report: `.ignored/TKT-PU3HUR-sweep-perf-report.md`.

### Findings
- **Both candidate scans are flat with dataset size** — ~1–3ms at 1M rows,
because they are `LIMIT $Batch` index scans over `entities_updated_at_idx` /
`relations_updated_at_idx`, with index-only LATERAL latest-version probes.
EXPLAIN (ANALYZE) at 1M confirms Index Scans, **zero seqscans**. The relation
scan TKT-92JL8P added costs ~0.4–1.2ms.
- **Advisory-lock hold ~2.7ms** at 1M (acquire 52µs + scans 2.6ms). A full tick
(scans + 1000 version INSERTs) was 136–258ms wall, dominated by the INSERTs, not
the scans — fine under a multi-minute Interval.
- **Invariants confirmed**: `rela_seq` unchanged (+0) while `version_seq` advanced
(+1000) → change-feed watermark unaffected by version volume.

### Recommendations
1. No index/query change needed — ship as-is.
2. Batch (not Interval) is the drain-rate knob: to drain a bulk-import backlog
faster, raise Batch (scan cost barely moves, amortizes the lock acquire). Worth
a one-line doc note in the postgres-backend guide.
3. If a future huge simultaneous-change backlog makes per-tick INSERT cost a
problem, batch/COPY the version inserts — not needed now.

### Follow-up (optional, low priority)
Add the doc note (#2) about Batch as the drain knob. The load-test harness is
left in the tree (uncommitted) for reuse; commit it with the e2e/perf follow-up
if desired.
