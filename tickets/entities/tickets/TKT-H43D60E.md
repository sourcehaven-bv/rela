---
id: TKT-H43D60E
type: ticket
title: 'Flows M1h: retention of finished runs'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Split from TKT-FWQZGAM. Retention for finished runs: archive or delete the
`_flow-run` headers of finished runs after the flow's `retention.after`. The
flow worker runs the sweep under its leader lock, capped per tick.

## Acceptance

- A finished run's header is archived or deleted after `retention.after`,
on fs, postgres and sqlite, and an unfinished run is never touched.
