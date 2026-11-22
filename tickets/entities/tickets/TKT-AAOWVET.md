---
id: TKT-AAOWVET
type: ticket
title: Audit record appends
kind: enhancement
priority: medium
effort: s
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

Split from TKT-EV8U8T4 ("decide whether record writes go to the audit log").
Appends are not audited; only changes to mutable rows are. A row stores only the
user who appended it, so the tool, the trigger (scheduled task, script) and the
asserted identity (`run_as`, verified JWT) are lost, and a refused append leaves
no trace.

The original reason was volume. An append already takes a batch of up to
`records.MaxBatch` rows, so one audit line per append call costs one line per
request, the same as an entity write.

## Scope

- Every successful `records.Service.Append` writes one `append-records` audit
line, for every record type. The line names the type, the row count, the owners,
the first and last row id and, for a ledger, the group keys.
- The line is written after the append commits, as for a change to a mutable
row. Writing record audit lines inside the record transaction is TKT-RS31NGP.
- A refused append on a readable owner writes a `denied-write` line, as a
refused change to a mutable row does. A hidden owner stays silent.
- GUIDE-records tells high-volume writers to batch.

## Acceptance

- One append of N rows writes exactly one audit line.
- A refused append is audited as denied; a hidden owner is not.
- The audit line carries the trigger and asserted-by of the ctx.
