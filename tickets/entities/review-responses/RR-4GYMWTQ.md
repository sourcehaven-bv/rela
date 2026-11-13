---
id: RR-4GYMWTQ
type: review-response
title: The clock accepted dates outside the time range
finding: t.clock and t.advance accepted years or durations that overflow time.Time.
severity: minor
resolution: The clock is limited to 1678-2261; advance rejects NaN and overflow. TestClockAndRandomBounds covers it.
status: addressed
---
