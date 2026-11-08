---
id: RR-3FNL4AN
type: review-response
title: 'Design: validation is not all before any read'
finding: Bounds and relation-meta checks run inside compile, after earlier ranges have been read.
severity: significant
resolution: Bounds are checked up front with store.PlanElementRange over an empty scope; type names against the metamodel. A meta-gated relation type fails the whole buffered request with 422, so nothing partial leaks; the check is principal-independent.
status: addressed
---
