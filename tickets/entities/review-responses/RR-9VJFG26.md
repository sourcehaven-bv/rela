---
id: RR-9VJFG26
type: review-response
title: Rows linked before 9b have no last-sync time
finding: The before-last-sync check needs a sync time on the ledger row; rows from 9a have none.
severity: minor
resolution: The check is skipped while the row has no sync time; the next recorded base sets it.
status: addressed
---
