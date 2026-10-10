---
id: TKT-2U059N
type: ticket
title: One affordance resolver and relation lookup for dataentry and appbuild
kind: refactor
priority: low
effort: m
status: backlog
---

## Description

Deferred from TKT-0XL8MF. dataentry builds its own affordances resolver
(affordances_stub.go) beside appbuild's buildFieldPolicy, and
storeRelationLookup exists in both packages. Hoist the lookup into
internal/affordances and let dataentry take the shared resolver. Both copies
swallow store iteration errors and answer no edge, which fails open for a `when:
not has_relation(...)` predicate; fix that while consolidating.
