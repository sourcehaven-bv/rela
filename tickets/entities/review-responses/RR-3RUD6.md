---
id: RR-3RUD6
type: review-response
title: 25-hour lookback window breaks weekly schedule detection
finding: prevScheduleTime only looks back 25 hours. Weekly schedules (0 9 * * 1) will never detect missed runs if scheduler was down for >25h.
severity: significant
resolution: Changed lookback from 25h to 8 days (maxLookback constant). Added test for weekly schedule detection
status: addressed
---
