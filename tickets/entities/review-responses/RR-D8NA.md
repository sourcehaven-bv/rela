---
id: RR-D8NA
type: review-response
title: Timezone inconsistency between time.Now() and date-only parsing
finding: time.Now() returns local time, time.Parse date-only returns UTC. days_since can be off by one day near midnight. Parse date-only strings in local time or truncate to midnight.
severity: significant
resolution: parseDate now uses time.ParseInLocation with time.Local for date-only strings. rrule_next uses UTC internally for RRULE comparisons via parseDateUTC.
status: addressed
---
