---
id: TKT-NLM6EH
type: ticket
title: 'Computed properties in schema.yaml: derived, non-editable, stored and indexed, with chained derivation and cycle detection'
kind: enhancement
priority: medium
effort: l
status: review
---

## Problem

Entity properties are currently authored. `schema.yaml` needs a way to declare
an entity-local derived value that is persisted, searchable, filterable,
sortable, and usable by ordinary views exactly like an authored property.

The motivating case is deriving `next_occurrence` from an RRULE and a start
date, while the general mechanism must also support arithmetic and string
composition.

## Acceptance criteria

1. Entity properties accept a typed `computed:` expression over properties of
the same entity.
2. Computed properties are read-only through data-entry, CLI, MCP, and Lua
mutation paths.
3. Computed values are materialized before persistence and therefore reach the
normal store observers/search index.
4. Chained derivations evaluate in dependency order.
5. Self-references and indirect cycles fail project/schema validation.
6. Expression changes participate in schema-shape drift detection.

Relation rollups/aggregates remain out of scope.

## Revised expression-engine decision

After inspecting rela's mini Lua-compatible interpreter, use the existing
`internal/predicate` typed IR rather than full Lua or an explicit `depends_on:`
list. Extend it with context profiles, typed scalar arithmetic and
concatenation, exact static property-reference extraction, and conservative
SQL-portability metadata.

This keeps dependency inference and cycle detection structural, gives future
database-query lowering one shared typed IR, and permits context-specific
capability sets. Computed expressions remain pure: no statements, dynamic
property access, store/relations, filesystem, network, or mutation handles.
Host-only functions such as `rrule_next` remain valid at write time but mark a
program non-portable instead of changing its semantics.

Clock-dependent functions capture write time. Existing calendar/feed paths keep
their live RRULE behavior rather than treating a stored snapshot as
authoritative. Changing a computed expression reports schema drift; bulk
recomputation is an explicit operator migration/touch and is not automated by
this ticket.
