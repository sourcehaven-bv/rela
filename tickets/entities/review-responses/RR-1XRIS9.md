---
id: RR-1XRIS9
type: review-response
title: Number and date semantics across backends
finding: 1.0 vs integer, Go float64 precision, datetime offsets and empty values could differ per backend.
severity: significant
resolution: 'One rule: a number is an integral JSON number in int64 range (1.0 counts everywhere), a string is non-empty and compared byte-wise; empty or missing values are skipped. Datetimes compare as text, documented (store them in one offset). Pinned in storetest.RunRollupTests against the generic reference on every backend.'
status: addressed
---
