---
id: RR-3J4WHHX
type: review-response
title: Overdue rows can push the for window out of the list
finding: An ascending sort loads past-dated rows first. More than max_rows overdue rows leave no day sections.
severity: significant
resolution: Documented with the advice to filter old dates out with a condition (as for relative buckets).
status: addressed
---
