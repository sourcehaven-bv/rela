---
id: RR-TQ2Y6O0
type: review-response
title: Tenant-mode shutdown is slow
finding: Nothing ends tenant SSE streams before srv.Shutdown, and residents close serially.
severity: minor
reason: Affects only shutdown latency and exit code; tracked in TKT-N3X235N.
status: deferred
---
