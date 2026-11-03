---
id: RR-O1H2ML
type: review-response
title: Wire shows due_at while timers are suspended under a degraded config
finding: 'RunRound does nothing under a degraded snapshot, but the view still derives and serves due_at, so readers see deadlines pass without a move. Plan change: omit due_at (keep timers) when the snapshot is degraded, or document it in the guide''s Reloading section and add a row to TestChannelsHandler_SessionTimersInReads.'
severity: nit
resolution: 'Plan updated: sessionView takes the degraded flag and omits due_at (timers still listed) under a degraded config; AC 15 adds the row; the guide''s Reloading section documents it.'
status: addressed
---
