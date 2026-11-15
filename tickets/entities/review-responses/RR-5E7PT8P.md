---
id: RR-5E7PT8P
type: review-response
title: Equivalence of windowed and full reads is not tested directly or on other backends
finding: No test compared executeQueryWith with firstN against the full read, and the tests run on memstore only.
severity: minor
resolution: 'TestFirstN_SameLeadingRowsAsAFullRead compares both reads under a role reached through a member-of walk: one type, a pushed filter, a Go-only filter, two types where the first runs out, and fewer matches than n. Running it on sqlite and postgres is TKT-LA7R91D.'
status: addressed
---
