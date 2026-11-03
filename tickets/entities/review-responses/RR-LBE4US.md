---
id: RR-LBE4US
type: review-response
title: decisionCause copies the held list on every recompute
finding: channelView.ts:426 `[...messages].reverse()` allocates a copy of up to 500 messages each time messages or the head change, and the head changes on every frame. Use a backward index loop.
severity: nit
resolution: decisionCause now walks the list with a backward index loop instead of copying it.
status: addressed
---
