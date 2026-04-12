---
id: RR-YRLY6
type: review-response
title: "dtstart not shown for YEARLY frequency"
finding: |
  Without dtstart, the rrule library defaults to current moment for first occurrence. User has
  no control over which year the first occurrence happens.
severity: significant
status: wont-fix
reason: Pre-existing behavior that applies to all frequencies, not specific to this change. The rrule library handles this correctly by defaulting to now. Out of scope for this ticket.
---
