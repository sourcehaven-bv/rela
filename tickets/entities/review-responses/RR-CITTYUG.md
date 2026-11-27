---
id: RR-CITTYUG
type: review-response
title: fs and memory keep the edge when a trigger set fails
finding: On the fs tier a refused set returns an error after the edge is written.
severity: significant
resolution: Kept by design for the fs tier, which has no rollback\; GUIDE-flows.md states it.
status: addressed
---
