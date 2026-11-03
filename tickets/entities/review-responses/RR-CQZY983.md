---
id: RR-CQZY983
type: review-response
title: Update always reads the pile first
finding: Service.Update calls GetPile on every update to check the lock.
severity: nit
reason: One extra read per rename is the cost of the lock check.
status: wont-fix
---
