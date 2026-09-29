---
id: RR-WSRFUP
type: review-response
title: v8 rung memory grows with table size
finding: normalizeColumn collected every changed row of a table and ran one unprepared exec per row.
severity: minor
resolution: Reads pages of 5000 rows by rowid and reuses one prepared UPDATE.
status: addressed
---
