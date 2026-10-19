---
id: RR-TA6UZE
type: review-response
title: Pending overflow flags the type of the incoming event, not the type that filled the buffer
finding: 'internal/realtime/trigger.go:172-177. When npending reaches MaxTriggerPending, the next Note marks c.EntityType full and drops that type''s ids, even if another type holds almost all 8192 entries. A single event of a quiet type then forces a full scan of that type, while the noisy type keeps its full list. Fix: when the cap is hit, flag the type with the most pending ids (or every type with pending ids) as full.'
severity: minor
resolution: On overflow the type that holds the most pending ids is flagged for a full compare, and the incoming change is kept. TestTriggers_OverflowFlagsTheBusiestType pins it.
status: addressed
---
