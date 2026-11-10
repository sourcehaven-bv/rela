---
id: RR-GCDTUIW
type: review-response
title: Runs listing is unpaged
finding: FlowControl.Runs loads every readable run header in one pass.
severity: minor
reason: Paging belongs with retention and the inbox in TKT-FWQZGAM, which bound how many headers exist. M1d volumes are small.
status: deferred
---
