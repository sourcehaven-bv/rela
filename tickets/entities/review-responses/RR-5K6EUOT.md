---
id: RR-5K6EUOT
type: review-response
title: Command payload loads bodies before filters
finding: loadRowContent runs for every readable row before applyFilters drops some.
severity: minor
reason: Bodies must load before redaction so content redaction applies\; reordering is part of paging the command payload, which TKT-D3RAB0E lists.
status: deferred
---
