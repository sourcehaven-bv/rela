---
id: RR-YRLY1
type: review-response
title: "Partial selection behavior unspecified"
finding: |
  Plan lists "month without day, day without month" as an edge case to test but doesn't specify
  the expected behavior. FREQ=YEARLY;BYMONTH=3 without BYMONTHDAY means "every day in March"
  which is almost certainly wrong. Plan should specify: require both month AND day, or emit neither.
severity: significant
status: addressed
resolution: Updated plan to require both month and day to be set before emitting BYMONTH/BYMONTHDAY.
---
