---
id: AM-relation-delete-addresses-the-tail
type: automated-measure
title: Relation delete addresses the tail, never a different face's edge
description: Store conformance case asserting DeleteRelationState removes the edge with exactly the given tail and leaves same-triple siblings on other tails standing. Guards the class where dropping a tail silently re-addresses a different relation instead of failing.
kind: test
location: internal/store/storetest/states.go
status: active
---

## What it guards

A relation's tail is part of its IDENTITY — two edges on one triple with
different tails are two relations. But `DeleteRelation`'s signature cannot
express a tail, so a caller holding a state-tailed edge that drops it does not
delete "approximately the right edge": it deletes the DEFAULT face's edge and
reports success.

That is not a hypothetical — it shipped in the copy kernel (BUG-CPYTAIL) and
was silent in all three backends.

## Shape

`DeleteRelationStateAddressesTheTailNotTheTriple` seeds one edge per tail on the
same triple, deletes the non-default one, and asserts the default-tail sibling
SURVIVES — the survivor assertion is what makes it capable of failing.
`DeleteRelationStateNotFound` asserts a missing tail is absent rather than
"close enough", and that the miss did not delete the default edge.

Implemented in TKT-C1XUA8 PR-D; runs against fsstore, memstore and pgstore via
`storetest.RunAll`.
