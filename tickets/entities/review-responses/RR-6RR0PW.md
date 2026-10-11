---
id: RR-6RR0PW
type: review-response
title: Every 409 was treated as a version conflict
finding: busy and migration_not_started 409s set conflict, which was never reset, so Save stayed disabled until a reload.
severity: significant
resolution: Only a conflict problem type sets conflict; busy, migration_not_started and 503 show the server message; a successful check resets it.
status: addressed
---
