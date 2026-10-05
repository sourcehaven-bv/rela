---
id: TKT-0ZXW0Z
type: ticket
title: List page write affordances resolve relation-conferred roles per row
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A list page whose rows are readable through a relation-conferred role
(`role_relations:`) costs one `GetRelation` per row per role relation. At 50
tickets with one conferring `watches` edge each, the list does 300 `GetRelation`
reads (6 per row); at 10 rows it does 60. The read gate itself is one
`MatchingFaces` per batch; the growth comes from the per-row `_actions` write
affordances, which resolve the target-scoped roles through
`acl.StoreGraph.HasEdge` (`internal/acl/resolver.go`) once per row and verb.

This violates the TKT-1U8XYN rule that collection reads are batched per page.
Found while writing `TestQueryBudget_ScopedVerdictListPageIsSizeIndependent` for
TKT-7IZHP0; the behaviour is the same on `faces-intrinsic` before that change.

## Approach

Batch the conferred-role lookup for a page: one `RelationQuery` over (member
set, role-relation types, row ids) that yields the conferring edges, then answer
each row from that map. Then tighten the budget test to assert total reads are
size-independent.
