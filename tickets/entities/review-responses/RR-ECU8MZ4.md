---
id: RR-ECU8MZ4
type: review-response
title: flow.Reminder unexported fields cannot be set by realtimecfg
finding: The loader in realtimecfg sets SessionReminder.in and inSet. Once they live in flow.Reminder, realtimecfg cannot set them.
severity: significant
resolution: 'Addressed in plan: flow.NewReminder builds the value; Active and In move with the fields.'
status: addressed
---
