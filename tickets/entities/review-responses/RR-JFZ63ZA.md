---
id: RR-JFZ63ZA
type: review-response
title: pgpiles MarkRead and PruneItems check ownership outside a transaction
finding: ownedBy and the write run as separate statements.
severity: nit
reason: 'Harmless: a deleted pile cascades its items so the write then touches nothing.'
status: wont-fix
---
