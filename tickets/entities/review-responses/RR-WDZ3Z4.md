---
id: RR-WDZ3Z4
type: review-response
title: first() lacks an id check
finding: A reader that drops IDs would let first() return any row.
severity: minor
resolution: first() now requires the row id to be one of the queried ids.
status: addressed
---
