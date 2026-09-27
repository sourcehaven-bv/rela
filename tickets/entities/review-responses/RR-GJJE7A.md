---
id: RR-GJJE7A
type: review-response
title: Results now stream
finding: The SQL path yields while rows are open, holding a read transaction or the pinned Tx connection.
severity: nit
resolution: Row queries are materialized before yielding (pages are bounded by Limit, and naive collected as well).
status: addressed
---
