---
id: RR-OKQ3LF
type: review-response
title: Datetime shift uses a different frame than the chart
finding: applyDayDelta shifted in the browser zone and wrote UTC, while the server draws a datetime's day in its own offset; values near midnight landed on another day.
severity: significant
resolution: New shiftStored shifts the date part and keeps time and offset verbatim, matching ganttDateString. Tests with +01:00 and -05:00 values near midnight.
status: addressed
---
