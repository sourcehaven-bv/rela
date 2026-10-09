---
id: RR-FI7XAU
type: review-response
title: Rows whose capture always fails stay candidates and can starve the batch
finding: A row whose capture errors (for example legacy properties that fail to unmarshal) returns before the write-back and stays at the front every tick; a full batch of them brings the starvation back with only per-row warnings.
severity: significant
resolution: 'Each tick phase now reports a full batch in which no row was resolved as a stalled sweep warning (noteBatch), and the comments that called the gate exact name this exception. Ordering failed rows to the back was not added: the failure predates this bug and needs a fix to the row itself.'
status: addressed
---
