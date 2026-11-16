---
id: RR-97V0ZHJ
type: review-response
title: Record writes are not in the audit log
finding: Appends and reversals bypass entitymanager, so the audit log never sees them.
severity: minor
reason: 'Documented in GUIDE-records: each row records recorded_by and recorded_at. Whether to audit record writes is tracked in TKT-EV8U8T4.'
status: deferred
---
