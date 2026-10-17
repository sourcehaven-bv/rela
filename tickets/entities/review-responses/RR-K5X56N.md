---
id: RR-K5X56N
type: review-response
title: Ceiling warning has no rate limit
finding: Many over-ceiling requests produce one warning each.
severity: nit
reason: Each line marks one expensive request; volume is bounded by request rate, and the access log has the same property.
status: wont-fix
---
