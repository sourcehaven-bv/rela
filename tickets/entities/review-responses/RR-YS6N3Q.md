---
id: RR-YS6N3Q
type: review-response
title: Configure save can lose config-changed
finding: Retire closes the bus before broadcasting config-changed; if a closed bus channel ends the SSE loop, select may return before writing config-changed and the tab keeps a stale schema (TKT-F5NGMG).
severity: critical
resolution: 'Subscriptions are never closed by the bus (wake channel never closed); only a closed broker channel or ctx ends the stream. AC6 test: Retire with a connected client, config-changed is the last frame.'
status: addressed
---
