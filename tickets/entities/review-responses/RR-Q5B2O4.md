---
id: RR-Q5B2O4
type: review-response
title: Timer-to-timer chains wait a full ceiling per hop
finding: 'After a fire, the new state''s due time can already be past (a from: session timer whose anchor has passed). TimerReport.Next only takes due times after now from the pre-write listing, and the service/system-writer noteDue does not cover the round''s own writes, so the next hop fires only at the 60s ceiling. Plan change: after each SetSession, read step.Outcome.Session.NextDueAt; if it is at or before now set More, else fold it into Next. Add TestTimers_ChainFiresWithoutCeiling.'
severity: minor
resolution: 'Plan updated: each SetSession outcome''s NextDueAt feeds the round: at or before now sets More, later folds into Next; AC 11 TestTimers_ChainFiresWithoutCeiling added.'
status: addressed
---
