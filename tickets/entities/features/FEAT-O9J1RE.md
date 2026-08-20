---
id: FEAT-O9J1RE
type: feature
title: Schema data migration system
description: 'Operators can evolve schema.yaml safely: rela detects whether stored data is compatible with the new schema (additive/drift changes auto-adopt with notices), and for genuine shape changes provides generated, reviewable, declarative migrations (rela migrate gen / rela migrate data) with a Lua escape hatch. Schema-orphaned data is cleaned by an audited periodic GC sweep after a grace period, or immediately via explicit migration delete steps.'
status: implemented
summary: Schema shape changes are detected, classified, and migrated with generated declarative migrations, a Lua escape hatch, and grace-period GC of orphaned data
---

Operators can evolve `schema.yaml` safely: rela detects whether the stored data
is compatible with the new schema (additive/drift changes auto-adopt, with
notices), and for genuine shape changes provides generated, reviewable,
declarative migrations (`rela migrate gen` / `rela migrate data`) with a Lua
escape hatch for arbitrary transforms. Schema-orphaned data is cleaned up by an
audited periodic GC sweep after a grace period, or immediately via explicit
migration delete steps.
