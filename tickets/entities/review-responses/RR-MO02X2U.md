---
id: RR-MO02X2U
type: review-response
title: Fix covers fs only
finding: sqlitestore and pgstore still load every row per create.
severity: minor
reason: Rows are cheap there and the report was fs. TKT-69TDK8 already covers pgstore and short ids.
status: deferred
---
