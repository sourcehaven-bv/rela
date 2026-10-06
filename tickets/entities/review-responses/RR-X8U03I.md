---
id: RR-X8U03I
type: review-response
title: SSE wall_ms is stream lifetime
finding: The record for event streams is written on disconnect, so wall_ms measures how long the tab was open.
severity: minor
resolution: 'Documented in the guide: leave the stream paths out of latency reports (perf-report already does).'
status: addressed
---
