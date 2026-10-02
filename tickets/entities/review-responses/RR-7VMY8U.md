---
id: RR-7VMY8U
type: review-response
title: Snapshot guidance for Tx is wrong
finding: pagedseq.go said a caller needing a consistent view should iterate inside a read Tx. pgstore Tx is READ COMMITTED and takes the write advisory lock.
severity: significant
resolution: Comment now states that iteration is not a snapshot on pgstore, that Tx does not restore one, and that callers must not depend on one.
status: addressed
---
