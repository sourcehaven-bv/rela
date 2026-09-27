---
id: RR-Y3FXUP
type: review-response
title: OFFSET without LIMIT is invalid in SQLite
finding: pg emits OFFSET alone; SQLite needs LIMIT -1 OFFSET ?.
severity: minor
resolution: Builder emits LIMIT -1 when only Offset is set; a paging test covers Limit 0 with Offset > 0.
status: addressed
---
