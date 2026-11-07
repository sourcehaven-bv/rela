---
id: RR-6N6Z8SB
type: review-response
title: state column written but never read
finding: flow_runs.state was stored but no index or query used it.
severity: nit
resolution: Dropped the column from migration 0042, the sqlite DDL and the queries.
status: addressed
---
