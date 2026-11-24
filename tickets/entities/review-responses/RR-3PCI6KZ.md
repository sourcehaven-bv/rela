---
id: RR-3PCI6KZ
type: review-response
title: Concurrent moves lost updates
finding: The lock covered only the Put and the new order was built from data read outside it.
severity: minor
resolution: Move reads and writes the stored order under one lock.
status: addressed
---
