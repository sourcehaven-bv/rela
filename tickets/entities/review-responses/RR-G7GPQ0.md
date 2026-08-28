---
id: RR-G7GPQ0
type: review-response
title: First row of new IsDue test duplicated an existing assertion
finding: TestScheduleIsDue_weekday_notISOWeekBased's first case used the identical Thu 2026-04-09 → Fri 2026-04-10 timestamps already asserted in TestScheduleIsDue_weekday_friday, adding no coverage; only the Sat→Mon row discriminated against an ISO-week implementation.
severity: minor
resolution: First row changed to lastRun Mon 2026-04-06 09:00 → now Fri 2026-04-10 08:00 (both 2026-W15, due) — a same-ISO-week pair not pinned elsewhere.
status: addressed
---
