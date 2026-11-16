---
id: RR-X78QOGW
type: review-response
title: Empty group read or reversed the whole ledger
finding: groupRows with an empty group sent no group filter, so _transaction loaded every row and _reverse could negate the whole ledger.
severity: significant
resolution: Transaction, Reverse and groupRows refuse an empty group, and groupRows refuses more than MaxBatch rows. TestLedger_EmptyGroupRefused and the handler test.
status: addressed
---
