---
id: RR-XGVU1GS
type: review-response
title: audit-read record is best-effort
finding: The audit-read record goes on the sink queue and may be dropped.
severity: minor
resolution: Documented in the handler godoc and the ACL guide.
status: addressed
---

The audit-read record goes on the sink queue and may be dropped.
