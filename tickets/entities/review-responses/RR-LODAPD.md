---
id: RR-LODAPD
type: review-response
title: A panic in Tx's fn permanently poisons the store and commits the rolled-back write
finding: Tx registered `defer conn.Close()` but no deferred ROLLBACK. On a panic, conn.Close() returns the driver connection to the pool with BEGIN IMMEDIATE still open; database/sql hands it out again, so every subsequent Tx fails with 'cannot start a transaction within a transaction' AND the uncommitted write becomes durable and visible. A panic in an automation, validator or observer inside fn would therefore both commit data reported as rolled back and take the store down until process restart. Recovery requires a restart.
severity: critical
resolution: 'Added a deferred rollback guarded by a `committed` flag, running on context.WithoutCancel so a cancelled ctx cannot skip it. Independently reproduced the mechanism in a standalone program before fixing: 10/10 subsequent connections poisoned, leaked row visible. Added storetest.RunTxAbnormalExitTests/PanicInFnLeavesStoreUsable to the SHARED suite so every backend is held to it; verified non-vacuous by reverting the fix (fails with ''a panicking transaction must not commit its writes'').'
status: addressed
---
