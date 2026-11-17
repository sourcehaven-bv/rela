---
id: RR-UUT8R23
type: review-response
title: Redundant head query per appended row
finding: Each append re-read the head inside the transaction.
severity: minor
resolution: The chain type tracks the head across a batch.
status: addressed
---
