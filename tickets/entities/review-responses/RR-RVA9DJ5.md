---
id: RR-RVA9DJ5
type: review-response
title: Comments name the wrong stage; test-only wrapper in production
finding: peerkey_edge.go said only stampAuditPrincipal sets the marker; production called stampIdentity and kept stampAuditPrincipal as a test-only wrapper.
severity: minor
resolution: Merged into one function stampAuditPrincipal(next, resolve, peers); test callers pass nil. Comments now say the marker is package-private and set only by peerKeyEdge.serve.
status: addressed
---
