---
id: RR-B77M1C
type: review-response
title: The no-effect warning conflicts with Q5's reasoning and fires during migration
finding: 'AC 9 warns "offer: transitions has no effect: every message type a transition is keyed by declares in:". Q5 argues that an explicit in: equal to the derived one is harmless and lets an operator be explicit. An operator who sets offer first and drops the copied in: entries one by one sees this warning until the first one is gone. Keep the warning, because a key that does nothing is worth flagging, as grants.go:59 does for run_as. Reword it to say the fix in both directions: "drop the in: that only repeat on: keys, or drop offer".'
severity: minor
resolution: 'Kept the warning and reworded it to name the fix in both directions: "drop the in: that only repeat on: keys, or drop default_in". Q5 now explains why the warning stays while an equal in: is not flagged.'
status: addressed
---
