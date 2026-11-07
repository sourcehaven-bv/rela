---
id: RR-KS8ZG4Z
type: review-response
title: 'Security: foldable predicate can be NULL'
finding: A grant condition over an absent property can evaluate to NULL, so NOT ok could miss a row and a CASE could list its stored hash.
severity: significant
resolution: 'Plan changed: ok is COALESCE(expr, FALSE) computed once and pending is NOT ok of that value; tests assert Count == folded + Pending and cover a condition over an absent property.'
status: addressed
---
