---
id: FEAT-T3EF5A
type: feature
title: User-data migration system for metamodel evolution
summary: 'Operator-run data migrations that follow user metamodel changes: rename properties/types/enum values/faces across entities, relations, and all three backends — distinct from internal/migration (rela''s own schema-YAML format migrations).'
description: 'Operator-run data migrations that carry the user''s entities/relations along with metamodel evolution: rename properties and entity types, remap enum values, set defaults, and re-key ids/faces/axes, across all three store backends with audit, versioning, and change-feed correctness. Distinct from internal/migration (rela''s own schema-YAML format migrations). Includes authoring model, execution path, applied-state bookkeeping, dry-run, and per-backend failure semantics; face/axis re-key for FEAT-9CD2MX becomes one migration op here (DEC-0VGTF3).'
priority: high
status: proposed
---

Decided 2026-08-19 (DEC-0VGTF3): rela needs a proper data-migration system for
the user's OWN schema evolution — today a metamodel edit (rename a property,
rename an entity type, change an enum value, later: rename a pointer / change
axes) silently strands the data written under the old schema. This is distinct
from `internal/migration`, which rewrites the schema-YAML *format* when rela
itself evolves; here the subject is the user's entities/relations.

Known design questions (research before planning):

- **Authoring model:** declarative ops (rename-property, rename-type,
map-enum-value, re-key-face, set-default) vs Lua scripts vs auto-derived diff
between metamodel versions. Precedent for declarative-only: world templates /
copy definitions deliberately reject expression languages.
- **Execution path:** through entitymanager (audited, validated, events,
versioned — but automations must be suppressed?) vs direct store writes (fast,
but bypasses audit/versioning/change feed). Interaction with pg content
versioning (does a migration mint versions per row?), the change feed, and the
audit log. Trust boundary: operator shell, like `history-purge` / `db migrate`.
- **Re-keying ops** (id prefix changes, face re-key, future axis changes)
touch PKs and filenames — need the store-level cascade treatment entity rename
already has, in all three backends.
- **Bookkeeping:** applied-migration state per project, dry-run default,
idempotency, partial-failure story per backend (pg tx vs fs best-effort).
- **Relationship to FEAT-9CD2MX:** face renames/axis changes become one
migration op; content-states v1 ships only detection (undeclared stored faces
→ load warning + analyze finding) and does not gate on this.
