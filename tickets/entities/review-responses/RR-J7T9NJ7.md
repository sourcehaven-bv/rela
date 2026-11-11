---
id: RR-J7T9NJ7
type: review-response
title: Ceiling imports store for InTx
finding: flowedits imports store only to skip its memo inside a Tx.
severity: nit
reason: store.InTx is the existing ctx marker for this; a second flowedits-specific marker would duplicate it.
status: wont-fix
---
