---
id: RR-QKCOJX6
type: review-response
title: 'Code: aggregate and key decoding duplicated in pg and sqlite'
finding: rangeSum and rangeEntryKey were copied between the two backends.
severity: nit
resolution: Moved to store.RangeSumFromAggregates and RangeEntry.SetRowKey, with unit tests; planRange copies replaced by elementnaive.Plan.
status: addressed
---
