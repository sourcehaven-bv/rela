---
id: FEAT-RELTRV
type: feature
title: Relation traversal in the condition language
summary: 'Conditions can filter on a related entity''s properties via related(entity, path, {constraints}) — valid Lua, compiled statically to a nested store.RelationPredicate so it pushes down to SQL as an EXISTS join rather than a per-row Go lookup.'
description: |-
    The condition language (internal/predicate) cannot reach the far side of a relation. Given `A -rel-x-> B`, no condition can filter on B's properties. The only relation-awareness today is has_relation / count_relations, which answer edge existence and counts but never load a neighbour.

    This feature adds traversal with SQL pushdown as the primary requirement: a traversal must compose into store.GraphQuery and be evaluated by the database, never row-by-row in Go.

    Surface syntax is the table form — ordinary, valid Lua:

        related(entity, 'caused-by', { type = 'ticket', status = 'done' })

    Chained hops take a path list. The form is recognised at COMPILE time and lowered to a dedicated traversal IR node (not a runtime host function), which is what keeps Program.SQLPortable true and leaves the record-return guard (RR-93UN) untouched.

    Semantics are EXISTS (any), returning bool. The `type =` key is a type ascription resolving multi-target relations — necessary because 32% of relations in the live schema are multi-target and their unions conflict on exactly the property operators most want (`status` is declared with a different enum type per entity type).

    Design, measurements and rejected alternatives are recorded in RES-RELTRV.
priority: medium
status: planned
---
