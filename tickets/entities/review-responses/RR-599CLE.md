---
id: RR-599CLE
type: review-response
title: AC4 short-circuit needs a defined decision point and a positive assertion
finding: 'All-DenyAll determination requires having computed the scope map (ReadQuery per type) — fine, but the AC should pin where the short-circuit happens and also assert the positive: a single-allowed-type query DOES invoke the searcher exactly once. As written AC4 only pins the negative.'
severity: minor
resolution: 'Plan rev 2 AC4: short-circuit decision point pinned (after scope construction, before any backend call) and positive control added — single-allowed-type query invokes the searcher exactly once.'
status: addressed
---
