---
id: RR-PJNKI5F
type: review-response
title: The event is judged against a row read before the transaction
finding: updateCore passed the old row read before the Tx, so a writer that committed in between was judged against a stale row.
severity: significant
resolution: priorRow reads the row on the transaction's view before the write, and the hook gets that row.
status: addressed
---
