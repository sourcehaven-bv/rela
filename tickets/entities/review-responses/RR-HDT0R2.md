---
id: RR-HDT0R2
type: review-response
title: Allowlist duplicated
finding: parseFlags and newAccessLogger each hard-coded stderr|syslog.
severity: nit
resolution: Single checkAccessLogDest used by both.
status: addressed
---
