---
id: BUGA-3L86DI
type: bug-analysis-checklist
title: 'Analysis: Badge colors never resolve when a property''s name differs from its custom-type name'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Confirmed by code inspection on `develop`:

- `internal/dataentry/app.go:853-882` — `buildStyleMap` keys the map by **type name**: explicit entries iterate `cfg.Styles` (whose keys `internal/dataentryconfig/validate.go:1368` `validateStyles` requires to be metamodel types), and auto-assign iterates `meta.Types`. Emitted as `styles` on `/api/v1/_config` (`api_v1.go:1913`, `apiwire/v1/responses.go:191`).
- `frontend/src/stores/schema.ts:204` — stored verbatim: `styles.value = configData.styles`.
- `frontend/src/components/common/Badge.vue:36` — `schemaStore.styles[props.property]` indexes by **property name**; miss → `badge--gray`.

Live repro: the in-tree `tickets/` project defines type `ticket_status`
(metamodel.yaml:18) used by property `status` (metamodel.yaml:333), and
`tickets/data-entry.yaml:6` styles `ticket_status:` — those badges render gray.
Only Badge.vue reads `schemaStore.styles`, so this is the single broken
consumer.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

1. **why1**: Badge.vue looks up `styles[propertyName]` but the server keys the map by custom-type name; the lookup only hits when a property is named identically to its type.
2. **why2**: The `styles` wire field is a bare `Record<string, Record<string, string>>` on both sides — the outer key's semantics (type name, enforced server-side by `validateStyles`) exist only in Go comments, so the SPA author plausibly assumed property names.
3. **why3**: Badge's unit tests seed `schemaStore.styles` keyed by property names (`status`, `priority`), encoding the wrong assumption; no test wires a realistic `_config` payload (type-keyed) through to a rendered badge, and no e2e asserts badge colors, so the miss ships silently as cosmetic gray.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach** (frontend-only): add `styleFor(property, value, entityType?)` to
the schema store, mirroring `labelsForProperty`'s resolution (schema.ts:100):
resolve the property def (entityType first, then first-wins scan of
entity/relation types), take `def.type` when it names a custom type, and look up
`styles[typeName]`; fall back to `styles[property]` (covers
property-name==type-name configs and keeps existing tests meaningful). Badge.vue
calls it instead of indexing `styles` directly. Server unchanged — its keying is
validated and correct.

**Regression test**: `badge-style-type-key-test` — Badge.test.ts case with
property `status` of custom type `ticket-status`, styles keyed by
`ticket-status`, asserting the configured class; plus
type-key-wins-over-property-key and fallback cases.

**Related areas**: grep shows Badge.vue is the only `schemaStore.styles`
consumer; kanban/list/detail/side-panel all render through Badge, so one fix
covers all surfaces.
