---
id: TKT-FKE3VP3
type: ticket
title: Push conditions over fields hidden on some rows
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

TKT-8HY05WB pushes a condition into the store only when every property it reads
is visible to the principal on every row, so the stored row and the redacted row
give the same answer. A condition over a field that `visible:` hides on some
rows stays on the Go path.

Rewrite each leaf against the compiled field scope
(`affordances.PolicyResolver.SearchFieldScope`): `(visible AND leaf) OR (NOT
visible AND leaf(nil))`, where `visible` is the grant's `store.GraphQuery`
predicate for the field and `leaf(nil)` is the leaf's constant answer on an
unset value (false for ordered and `==`, true for `~=`). The rewrite needs the
grant predicates expressible as a `store.Cond`.

## Acceptance

- A differential test: the rewritten store query and the Go evaluator over
redacted rows agree for random conditions and random grants.
- No value of a hidden field changes which rows a principal sees beyond what
the Go path already reveals.
