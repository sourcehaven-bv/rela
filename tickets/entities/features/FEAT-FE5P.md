---
id: FEAT-FE5P
type: feature
title: Reorderable relations
summary: Metamodel-declared ordering for relations so users can order related items in the data-entry UI
description: Relation types can declare an ordering property; data-entry surfaces drag/move controls and persists the order back to the relation.
priority: medium
status: proposed
---

## Goal

Allow users to express a meaningful order on related items (e.g., recipe steps,
chapter sections, prioritized requirements) and let the data-entry UI honor and
edit that order.

## Capability

- A relation type can be declared **orderable** in the metamodel by pointing at an ordering property on the relation.
- The data-entry UI renders related items sorted by that property.
- The UI offers drag/move controls; reordering writes the new value back via the existing relation update path.
- Manual edits to markdown files producing gaps or duplicates remain tolerated; ordering degrades gracefully.

## Relationship to other features

- Builds on relation properties (see `FEAT-jsjj`) — the ordering value lives in a typed property on the relation, not on the source or target entity.
- Complements the data-entry detail screen (`FEAT-KQ45P`) — orderable lists live in detail-view list sections.

## Out of scope

- Ordering of entities globally (independent of any relation).
- Cross-relation total order (mixing different relation types under one heading).
- A standalone "reorder via Lua API" — the relation update path is sufficient.

## Open design questions

These are tracked in the implementing ticket; they belong to design, not to the
feature definition itself:

- Underscore-convention property name vs user-named property.
- Outgoing vs incoming side semantics for the same relation type.
- Ordering scheme (dense integer + renumber-on-collision vs sparse/midpoint LexoRank).
- Polymorphic relations: ordering across mixed target types.
