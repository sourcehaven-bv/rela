---
id: TKT-H72Z2N
type: ticket
title: Split metamodel.Metamodel into focused types
kind: refactor
priority: medium
effort: l
started: "2026-10-11"
completed: "2026-10-11"
status: done
description: Move validation, projections, relation and property-type queries off Metamodel; delete duplicate getters; ratchet its plimsoll directive.
---

## Description

`metamodel.Metamodel` (internal/metamodel/types.go:22) has 34 methods, 32
exported, grandfathered by `//plimsoll:max-exported-methods=32` (cap 20). Split
its behaviour into focused types so it becomes mainly the decoded schema.yaml
plus a lookup catalog.

Origin: archplan thread th-0e0ed196.

## Scope

In scope:

- Delete getters that duplicate exported fields: `GetVersion`,
`GetNamespace`, `GetEntities`, `GetRelations`, `GetTypes` (24 call sites read
the field instead).
- Validation (`ValidateProperties`, `ValidateEntity`,
`ValidateRelationProperties`, `ValidatePropertyValue`, validation.go) to a
validator type built from the schema.
- Projections (`RenderProjection`, `ShapeProjection`) to their own type or
functions.
- Relation queries (`GetRelationLabel`/`From`/`To`, `InverseOwner`,
`ValidateRelation`, `InferEntityType`): to `RelationDef` where they need only
the definition.
- Property-type queries (`GetPropertyType`, `IsEnumType`, `GetTypeDefault`,
`GetPropertyDefault`, `IsPropertyRequired`, `ResolveWidgetFromType`) to a
type-catalog type.
- Ratchet the plimsoll directive down to the new count, ideally removing it.
- `EntityDef` (24 exported, directive at types.go:301) gets the same review.

Out of scope:

- The package rename to `schema` (TKT for the rename, after this).
- No behaviour change; YAML shape unchanged.
