---
id: RR-T1M860X
type: review-response
title: Read-back after a rolled-back batch can audit another caller's create
finding: failed() decides what stayed by reading each written id from the outer store after the Tx and the lock are released. On pg/sqlite a rolled-back sequential id is free again, so a racing create can take it; the batch then audits it under the wrong principal and runs its follow-ups. finishKept in linked.go has the same flaw.
severity: critical
resolution: CreateBatch and CreateLinked ask the store whether its Tx rolls back (store.TxRollsBack, new optional store.TxRollback). On pg and sqlite a failure returns the error with no read-back; the read-back stays only for fs and memory. TestCreateBatch_RolledBackIDTakenByAnother pins it.
status: addressed
---
