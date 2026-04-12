---
id: RR-YRLY3
type: review-response
title: "Stale state when switching away from YEARLY frequency"
finding: |
  selectedMonth and selectedDay retain values when switching to another frequency. If user
  switches back to YEARLY, stale values reappear.
severity: critical
status: wont-fix
reason: Matches existing weekday behavior — selectedDays is not cleared when switching away from WEEKLY. Keeping state prevents accidental data loss if user briefly switches frequency. The v-if guard prevents stale values from affecting the emitted rrule string.
---
