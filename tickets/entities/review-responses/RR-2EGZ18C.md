---
id: RR-2EGZ18C
type: review-response
title: Write lock held for up to 100 id scans
finding: createCore runs collectAllIDs per generated id under the deployment-wide lock; no budget test measures it.
severity: minor
reason: The scan is bounded by the 100-operation limit and runs on a single type; minting once per type is a separate refactor of createCore. Tracked in TKT-Y58LS2N.
status: deferred
---
