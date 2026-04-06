---
id: RR-CRM1
type: review-response
title: RRULE DTSTART override destroys rule semantics
finding: Setting opt.Dtstart = after means interval-based rules count from the wrong epoch. FREQ=WEEKLY;INTERVAL=2 fires on wrong weeks. BYMONTHDAY rules may also misbehave. Should preserve original DTSTART and only use After() to find next occurrence.
severity: critical
resolution: Don't override DTSTART. Reject INTERVAL > 1 without explicit DTSTART in the RRULE string. Users must anchor interval cadence explicitly.
status: addressed
---
