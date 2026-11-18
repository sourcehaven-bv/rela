---
id: RR-H886EAU
type: review-response
title: Failure paths keyed on the sink, not the store
finding: Soft delete and cascade failure paths checked auditsInTx instead of whether the store rolls back.
severity: minor
resolution: Guards use store.TxRollsBack in softdelete, recordPartialCascade, rename and conversion.
status: addressed
---

Soft delete and cascade failure paths checked auditsInTx instead of whether the store rolls back.
