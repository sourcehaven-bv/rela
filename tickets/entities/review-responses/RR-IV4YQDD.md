---
id: RR-IV4YQDD
type: review-response
title: Budget test pins memstore only
finding: TestQueryBudget_ListPathRollupsAreSizeIndependent counts memstore reads; the one-statement claim for pgstore and sqlitestore is not tested.
severity: minor
reason: Matches the existing roll-up budget tests which also pin memstore; the SQL backends issue one Rollup call per column by construction.
status: deferred
---
