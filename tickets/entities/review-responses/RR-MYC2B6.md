---
id: RR-MYC2B6
type: review-response
title: Bare-date rejection for datetime is unimplementable post-yaml-parse
finding: 'Plan rejects a bare date (2026-07-13) on a datetime prop. But ParseDateValue returns time.Time, and 2026-07-13T00:00:00Z (typed midnight) and 2026-07-13 (no time) both yield identical midnight time.Time (verified). Post-parse you cannot distinguish them; and yaml has already collapsed the lexical form to time.Time before validation runs, so the raw string isn''t even available on the file-load path. The rule can only be enforced on the raw JSON request string at the data-entry API boundary, giving inconsistent enforcement (API strict, file-load lax). Recommendation: DROP bare-date rejection; treat a bare date as midnight-UTC (lossless date->datetime widening, matches every other date system). This also removes the need for a date->datetime migration (finding 8).'
severity: critical
resolution: 'DROP bare-date rejection. Backend: a bare date on a datetime prop parses to midnight UTC (2026-07-13 -> 2026-07-13T00:00:00Z), stored verbatim, NO server-side local-tz guessing (backend is deliberately UTC; no such setting exists). Frontend date-shift concern confirmed real (2026-07-13T00:00:00Z displays as 2026-07-12 20:00 in America/New_York). Resolution: the WIDGET never emits a bare/midnight-only value - a user always picks a real time, so any shift is a meaningful true-instant shift. Bare/midnight values exist only via hand-edit/legacy; we accept them verbatim and DOCUMENT that pre-existing bare dates display at midnight-UTC (prior evening in western zones) rather than heuristically un-shifting them (can''t distinguish typed-midnight from bare-date). Lossless date->datetime widening; no migration needed.'
status: addressed
---
