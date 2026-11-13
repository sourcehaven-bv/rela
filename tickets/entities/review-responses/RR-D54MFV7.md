---
id: RR-D54MFV7
type: review-response
title: Malformed verdict now searches before failing
finding: A verdict that is neither allow-all, deny-all nor carries a query now runs the free-text search before scopedHeaders refuses it.
severity: nit
reason: No gate returns that shape\; the response is the same 500 and the search is already ACL-scoped.
status: wont-fix
---
