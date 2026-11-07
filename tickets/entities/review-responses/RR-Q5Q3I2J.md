---
id: RR-Q5Q3I2J
type: review-response
title: 'Code: ErrRangeTooLarge varies with backfill state for stored types'
finding: For a range of FoldStored types an unfilled element forces the list path so the same range can be refused during a backfill and answered after it.
severity: minor
resolution: 'Documented on ErrRangeTooLarge: it depends on scope and row count and for stored types on backfill state; neither is secret.'
status: addressed
---
