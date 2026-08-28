---
id: data-migration
type: concept
title: Schema-Driven Data Migration
summary: Shape-hash-keyed migration of stored content when schema.yaml changes
description: 'Evolving schema.yaml safely: a semantic ShapeProjection hash identifies the data-relevant schema, a compatibility classifier auto-adopts additive/drift changes at startup, operator-authored declarative migrations (with a pure-transform Lua escape hatch) transform content across incompatible changes, per-store applied state lives in state.KV, and a grace-period GC sweep cleans schema-orphaned data. Distinct from internal/migration (config YAML rewrites) and pgstore DDL migrations, which run before it.'
layer: core
status: draft
---

When `schema.yaml` changes shape (property renamed, type changed, enum values
remapped, entity type renamed), the stored entity/relation content must follow.
This concept covers:

- **Shape identity**: a semantic hash of the data-shape-relevant slice of the metamodel (`ShapeProjection`, sibling of `metamodel.RenderProjection` but including relation types/properties and defaults, excluding display/behavior config). Cosmetic schema edits do not change the shape hash.
- **Compatibility classification**: diffing two shape projections into tiers — *additive* (new type/optional property/enum value: auto-adopt silently), *drift* (deletions, new required property, delete+add pairs: auto-adopt with notices), *needs-migration* (type/format changes, narrowings, renames).
- **Migrations**: operator-authored files in `migrations/` keyed `from`/`to` shape hash, declarative steps (rename/map_values/convert/set_default/drop_*) with a pure-transform Lua escape hatch; generated as reviewable drafts by `rela migrate gen`.
- **Per-store applied state**: a marker in `state.KV` (`.rela/` on fs, `state_kv` per schema on postgres) recording the shape hash + projection the data conforms to.
- **GC sweep**: periodic, grace-period-backed cleanup of schema-orphaned data (deleted properties, entities/relations of unknown type), with a drift ledger of first-seen timestamps.

Distinct from the existing `internal/migration` package (config-file YAML
rewrites) and from pgstore DDL migrations — those remain separate layers that
run before data migration.
