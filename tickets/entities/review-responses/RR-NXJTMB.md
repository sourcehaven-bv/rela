---
id: RR-NXJTMB
type: review-response
title: Span is not clamped
finding: 'channelFields.ts:77 and RlChannelComposer.vue:264. A span outside 1..12 (if the server validation ever drifts) yields `grid-column: span 13` and implicit columns. Clamp with Math.min(Math.max(span, 1), 12) in resolveChannelFields.'
severity: nit
resolution: resolveChannelFields now clamps span to the range 1 to 12.
status: addressed
---
