---
id: RR-IIDMNN
type: review-response
title: A different action on a pending message answers action_taken instead of action_pending
finding: 'internal/realtime/actions.go:56-60 and channels.go:713 a none-to-done CAS racing a pending action fails with ErrConflict and maps to 409 action_taken. Fix: return a distinct ErrActionPending from the CAS when the current status is pending and map it to action_pending.'
severity: minor
resolution: CheckActionCAS returns ErrActionPending when the current action is pending so a lost race answers 409 action_pending; TestActionStoreError_PendingWinsRace and channelstest pin it (26a1cb8d8)
status: addressed
---
