---
id: RR-R2LABVS
type: review-response
title: Changed.State empty after a fork on the error path
finding: Decide's Target is empty for a fork, so Changed.State is empty when settle fails after a fork.
severity: nit
reason: An empty State is documented as "that place is gone"; after a fork the stepping token no longer exists.
status: wont-fix
---
