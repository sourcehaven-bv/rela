---
id: RR-NSHSC70
type: review-response
title: Idle decision is made before the transaction
finding: PrepareUpdate decides idle before the Tx; a run started by another writer in between misses this edit's event.
severity: minor
reason: The window is one write wide, and the next edit or step of the new run moves it. Closing it needs the run list inside the Tx on every write, which S3 removed for cost.
status: deferred
---
