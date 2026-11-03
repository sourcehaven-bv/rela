---
id: RR-CJAYKCA
type: review-response
title: WaitingMessages pairs Entries and waiting by index without a guard; second config snapshot
finding: '[security] Entries and waiting are paired by index with no length check, so a future filter would attach a message to the wrong row. The step also takes a second config snapshot while moderator comes from the first; a reload between the steps mixes them. Same root as the security reviewer''s time-of-check nit.'
severity: nit
resolution: InboxView keeps the config it was gated under; WaitingMessages renders nothing when the snapshot differs or when waiting and Entries differ in length. TestWaitingMessages_ConfigReloaded pins the reload case.
status: addressed
---
