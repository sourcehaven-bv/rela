---
id: RR-JR5CKO5
type: review-response
title: Every process runs the heal pass at once
finding: 'Each Open starts a healer: every server replica and scheduler and CLI command. They run the same batches together and contend on row locks; the first pass after the upgrade rewrites every row with index churn.'
severity: significant
resolution: 'Each pass runs on one connection holding a schema-scoped advisory lock (RELT); a process that cannot take it skips the pass. A pause between batches was not added: batches are 500 rows and the pass runs once in the background.'
status: addressed
---
