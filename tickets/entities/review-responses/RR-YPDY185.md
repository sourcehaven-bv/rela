---
id: RR-YPDY185
type: review-response
title: Scheduled pull sets triggered-by twice
finding: stampTaskAuditContext set schedule:<task> and the next line overwrote it.
severity: nit
resolution: pullRemoteTask calls stampContext once with remote:<name>.
status: addressed
---
