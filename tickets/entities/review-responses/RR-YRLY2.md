---
id: RR-YRLY2
type: review-response
title: "Day-of-month not validated against selected month"
finding: |
  Max=31 on the input allows invalid dates like Feb 31. The rrule library silently produces
  zero occurrences for impossible dates, so the recurrence would never fire.
severity: critical
status: addressed
resolution: Added maxDay computed that returns days-in-month. Day input uses dynamic :max and clamps value. Month change watcher auto-clamps day.
---
