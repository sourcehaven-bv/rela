---
id: RR-KJ7FLN
type: review-response
title: No ref backfill for existing type records
finding: Records written before migration 0027 have no refs, so a ref-to-string change after that passes the check.
severity: nit
reason: The branch is unreleased; only dev databases can hold records from before migration 0027.
status: wont-fix
---
