---
id: RR-W2JQKA
type: review-response
title: 'Minor: non-UTC datetime input silently UTC-converted; day() test-helper clarity'
finding: (a) formatDateTimeUTC calls .UTC(), so a hand-edited +02:00 datetime renders converted to UTC (correct per iCal, possibly surprising) — no action, just noted. (b) With Timed bool defaulting false, the day(m,d) midnight-UTC test helper stays unambiguously all-day, so the reviewer's ambiguity concern evaporates; add a dayTime(m,d,h,min) helper for timed test literals so intent is explicit.
severity: nit
resolution: Adopted. Timed bool (RR-NZ2I90) resolves the day() ambiguity for free. Add a dayTime(...) helper for timed test literals. Non-UTC conversion is correct-by-iCal; note it in the Event.Start/feed docs but no code change.
status: addressed
---
