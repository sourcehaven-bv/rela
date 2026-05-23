---
id: RR-CD4G
type: review-response
title: Rapid double-click on same checkbox causes silent ping-pong
finding: 'contentClick fires `void handleCheckboxToggle(idx)` without an in-flight guard. Rapid double-click queues two toggles; net effect: zero. User sees their clicks ''do nothing'' because state ping-ponged. Server''s writeMu serializes writes but doesn''t collapse them.'
severity: significant
resolution: 'Added `togglingIndices: Set<number>` to EntityDetail.vue. `contentClick` early-returns if the index is already in flight; `handleCheckboxToggle` adds the index in a try/finally so the entry is cleared even on error. Server-side serialization stays intact; client-side dedup prevents the queueing.'
status: addressed
---
