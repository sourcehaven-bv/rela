---
id: TKT-356XUF
type: ticket
title: 'Split metamodel.Metamodel part 2: relation and property-type queries'
kind: refactor
priority: medium
effort: m
started: "2026-10-11"
completed: "2026-10-12"
status: done
description: Move relation and property-type queries off Metamodel; review EntityDef; remove the Metamodel plimsoll directive.
---

## Description

Part 2 of TKT-H72Z2N. Part 1 (#1855) moved validation and the projections off
`metamodel.Metamodel` and deleted the field getters; it now exports 21 methods
under `//plimsoll:max-exported-methods=21`. This ticket moves the relation and
property-type queries, reviews `EntityDef` (24 exported, directive in types.go),
and ratchets both directives down, removing the Metamodel one.

Origin: archplan thread th-0e0ed196.

## Scope

In scope:

- Relation queries (`GetRelationLabel`/`From`/`To`, `InverseOwner`,
`ValidateRelation`, `InferEntityType`): to `RelationDef` where they need only
the definition, otherwise to a focused type.
- Property-type queries (`GetPropertyType`, `IsEnumType`, `GetTypeDefault`,
`GetPropertyDefault`, `IsPropertyRequired`, `ResolveWidgetFromType`) to a
type-catalog type.
- `EntityDef`: delete getters that duplicate exported fields; lower its
directive.
- Metamodel ends at 20 exported methods or fewer; remove its directive.

Out of scope:

- The package rename to `schema` (TKT-ZTGLU4).
- No behaviour change; YAML shape unchanged.
