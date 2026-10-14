---
id: RR-VPN5EZ
type: review-response
title: Calendar fails whole grid when one source's date filter is refused
finding: CalendarView sends filter[<date>][gte|lt] per source and loaded them with Promise.all, so one source refused with property_not_visible failed the whole calendar.
severity: significant
resolution: 'refetchGrid uses Promise.allSettled. A refused source shows its own error region (calendar-source-error) and the other sources still render; when every source fails the single calendar error stays. Tests: CalendarView per-source failures (two cases).'
status: addressed
---
