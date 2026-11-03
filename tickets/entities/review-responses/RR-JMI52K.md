---
id: RR-JMI52K
type: review-response
title: Cross-process precision claim is too pessimistic
finding: 'Plan lines 618-621 and 925-926 say "within a minute otherwise". first is at least 1m, and the leader lists due times up to now + TimerCeiling (timers.go:501). The leader''s next ceiling round therefore sees a slot set by another process and schedules slot + 30s. Precision is about 30s either way, plus drain rounds. Plan change: state "within 30 seconds, later only while a backlog drains" and drop the leader/non-leader split.'
severity: nit
resolution: 'Addressed: the claim is now "within 30 seconds, later only while a backlog drains", without the leader/non-leader split, with the ceiling-listing reasoning.'
status: addressed
---
