---
id: RR-NCBRP7
type: review-response
title: sqlite updated_at string comparison misorders trailing-zero timestamps
finding: time.RFC3339Nano trims trailing zeros, so string comparison in the sweep windows does not match time order; a row can miss one tick.
severity: minor
reason: Pre-existing and independent of this change. Filed as BUG-HEIAVS; needs a format change plus a data migration.
status: deferred
---
