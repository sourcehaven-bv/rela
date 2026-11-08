---
id: RR-166LREN
type: review-response
title: Confirmation does not show the target database
finding: The lock is per database; an operator with a different RELA_DATABASE_URL would drop a same-named schema elsewhere.
severity: minor
resolution: The prompt and result line name database, host, port and user (tenant.DescribeTarget, never the password).
status: addressed
---
