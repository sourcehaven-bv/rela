---
id: RR-92O6L6K
type: review-response
title: history-purge skips the generation guard when flows.yaml fails to load
finding: '[security] cli/history_purge.go: if LoadTriggers fails the rules are nil, so hasDeletedTriggers is false and purge skips writing the new generation that keeps a queued on.deleted job from reading the purged state. Fix: fail toward the guard and treat a load error as ''has deleted triggers''.'
severity: minor
resolution: PurgeDeletedJobStates takes the newGeneration decision; history-purge passes true when flows.yaml does not load (an extra generation costs one record). HasDeletedTriggers is exported for the caller; TestDeletedState_ErasedOnPurge has an unreadable-triggers case.
status: addressed
---
