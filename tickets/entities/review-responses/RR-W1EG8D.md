---
id: RR-W1EG8D
type: review-response
title: fs appends not serialized across processes
finding: a per-channel mutex is per process; CLI and server can assign the same seq and interleave lines; exactly-once promote has the same hole
severity: critical
resolution: 'Plan updated: .ignored/phase3-plan.md section Design review folded in (T1/T2) states the change; implementation tests pin it.'
status: addressed
---
