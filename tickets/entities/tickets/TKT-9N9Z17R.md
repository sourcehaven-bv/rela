---
id: TKT-9N9Z17R
type: ticket
title: Flow run read gate for inherited and face-split subject reads
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

TKT-BC5G8RP gates a `_flow-run` by its subject through `_carries`. The gate
folds each subject type's read query into an endpoint predicate with `gateHop`.
`gateHop` refuses a read query that uses `inherit_roles_through` or `Any`
branches (face-split role relations). Such a subject type drops its branch, so a
reader who reaches a subject only that way does not see its runs. This fails
closed but hides runs a reader should see.

Make endpoint predicates carry nested inheritance and branch disjunctions on
every backend, with storetest conformance, and drop the fallback.
