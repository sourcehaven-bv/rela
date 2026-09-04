---
id: TKT-OBC7QI
type: ticket
title: Move the migration MetamodelProvider implementation into internal/migration (SchemaAdapter); delete dead Metamodel.IsEnumType (32 → 25 exported)
kind: refactor
priority: medium
effort: s
tags:
    - tech-debt
status: backlog
---

Sub-ticket of [[TKT-N0IKN9]] — the `metamodel.Metamodel` exported-surface
offender (32 exported methods, line is 20; directive at
`internal/metamodel/types.go:22`).

## Diagnosis

`Metamodel` is not a god-object. It is a wide read API over a wide schema PLUS a
foreign interface implementation bolted on to satisfy one consumer.
`internal/migration/migration.go:14` already declares `MetamodelProvider` as a
correct consumer-side interface, and `dataentry_cleanup_test.go:11` already has
a `mockMetamodel` for it. Six methods exist on `Metamodel` purely so
`*Metamodel` structurally satisfies that interface — the consumer interface was
right; the implementation landed on the producer.

This is the named counterpart of the consumer-side-interfaces rule and it recurs
(appbuild.Services has the identical shape with the scheduler). Worth one
paragraph in `docs/architecture/consumer-side-interfaces.md`.

## What moves

Migration-only accessors from `internal/metamodel/schema_output.go`:
`GetPropertyType` (:33), `IsPropertyRequired` (:46), `GetPropertyDefault` (:59),
`GetRelationLabel` (:88), `GetRelationFrom` (:97), `GetRelationTo` (:106). Each
has 1-2 callers, all in `internal/migration`.

```go
// internal/migration/metaadapter.go — in the CONSUMER package.
// SchemaAdapter satisfies MetamodelProvider over a loaded *metamodel.Metamodel.
type SchemaAdapter struct{ Meta *metamodel.Metamodel }
func (a SchemaAdapter) GetPropertyType(entityType, property string) string { … } // 3-6 lines each over public fields
```

Call-site change: `migration.ApplyWithMetamodel(path, ft, fs, svc.Meta)` → `…,
migration.SchemaAdapter{Meta: svc.Meta})` in `internal/cli/migrate.go`.

`GetTypeDefault` (:72) and `ResolveWidgetFromType` (:117) each have a second,
non-migration caller — they stay for now (the SchemaOutput ticket takes them).

## Delete

`IsEnumType` (schema_output.go:80) — **zero callers**.

## Keep (deliberately)

- Core lookups (`GetEntityDef` 126 callers/22 pkgs, `GetRelationDef`,
`EntityTypes`, `RelationTypes`, `HasEntityType`, `ResolveAlias`).
- Value validation (`ValidateEntity` 14 callers/6 pkgs and friends) — schema
owns value semantics; a view would add a hop on a hot path for 4 slots.
- `RenderProjection`/`ShapeProjection` — two hashes with a load-bearing
difference documented in CLAUDE.md; a `Projections()` view saves 1 slot and
obscures it.

Ratchet `//plimsoll:max-exported-methods` at types.go:22 to 25.
