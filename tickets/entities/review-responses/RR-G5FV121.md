---
id: RR-G5FV121
type: review-response
title: Misleading error for a tenant with an empty DSN
finding: Postgres reported set RELA_DATABASE_URL for an empty tenant DSN.
severity: nit
resolution: OpenAndAssemble rejects an empty DSN with an error naming the tenant DSN.
status: addressed
---
