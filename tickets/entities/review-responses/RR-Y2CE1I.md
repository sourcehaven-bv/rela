---
id: RR-Y2CE1I
type: review-response
title: Differential test only on sqlite
finding: Three implementations (naive/pg/sqlite) stay aligned only if pg also runs the differential.
severity: significant
resolution: storetest.RunGraphDifferential runs in pgstore conformance (DB-gated) as well as sqlite.
status: addressed
---
