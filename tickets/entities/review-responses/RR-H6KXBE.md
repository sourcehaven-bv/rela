---
id: RR-H6KXBE
type: review-response
title: Trigger marks with outputs wait for the minute tick
finding: 'Approach 7 wakes the outputs runner only after a timer round with Marks > 0 (plan:379-382). Trigger posts now also write unclaimed marks with outputs (Approach 9) but TriggerReport has no mark count, so those writes lag up to a minute. Plan change: count marks in TriggerReport and wake on either, or state the asymmetry in the docs.'
severity: nit
resolution: TriggerReport.Marks added; the worker wakes the outputs runner after a timer or trigger round with Marks > 0 (Approach 12, AC 9).
status: addressed
---
