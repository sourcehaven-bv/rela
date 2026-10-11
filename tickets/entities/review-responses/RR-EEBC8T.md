---
id: RR-EEBC8T
type: review-response
title: Redundant pile reads on update and add
finding: Service.Update and the handler's addItems load the full pile with items only to check it exists; Update then reads it again.
severity: nit
resolution: Update reads the pile first only for a partial update. TestService_UpdatePartial.
status: addressed
---
