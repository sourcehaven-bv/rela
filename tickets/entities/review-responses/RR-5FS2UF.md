---
id: RR-5FS2UF
type: review-response
title: Inner cursor bound only added for paged world reads
finding: The inner id >= cursor filter is exact without a limit too but was added only when limit > 0.
severity: nit
resolution: The inner bound is now added whenever a cursor is present.
status: addressed
---
