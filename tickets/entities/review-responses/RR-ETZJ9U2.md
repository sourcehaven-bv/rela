---
id: RR-ETZJ9U2
type: review-response
title: Snapshot parameter is avoidable
finding: If flow.Read copies the stored Restarts and Reminders into the synthesized initial state, one Current value covers every read and callers cannot pass the wrong one.
severity: minor
resolution: 'Addressed in plan: flow.Read carries the stored Restarts, Reminders and the creation time on Current; engine functions take Current only.'
status: addressed
---
