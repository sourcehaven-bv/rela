---
id: RR-8QILNS
type: review-response
title: Same-property specs on different types produce duplicate index definitions
finding: Two types with the same properties get identical DDL under different names, doubling write cost.
severity: nit
reason: Mirrors pgstore's naming by spec; deduplicating by DDL would change both backends' naming and is not worth it for a rare shape.
status: wont-fix
---
