---
id: TKT-ZKMGUJ5
type: ticket
title: 'Flows: settle create starts off the request path and bound fan-out'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

A create that starts flow runs (TKT-I1OLJ3V) settles each run before the request
returns, for at most 30 seconds per run. A create that starts many runs holds
the request for a long time. The depth bound (8) limits how long a chain of
create starts gets, not how wide it fans out: one run that creates several
subjects starts a run for each.

Move the settle off the request path (for example onto the flow worker or
`jobs.Queue`), and decide whether a per-create or per-run cap on fan-out is
needed. Deferred review finding: RR-5V46IZQ.
