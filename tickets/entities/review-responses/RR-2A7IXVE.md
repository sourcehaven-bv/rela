---
id: RR-2A7IXVE
type: review-response
title: The control-table upgrade has no lock timeout
finding: The upgrade DDL waits behind any open transaction on the control table, and every resolve queues behind it.
severity: minor
resolution: controlDDLTx sets lock_timeout 5s after the advisory lock. The guide notes that older replicas ignore read_only during a rolling upgrade.
status: addressed
---
