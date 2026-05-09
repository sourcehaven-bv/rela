---
id: RR-UT6HS
type: review-response
title: entity_refs() must iterate per-type; ListEntities requires Type
finding: store.ListEntities (used at internal/lua/runtime.go:494) requires a non-empty Type. There is no 'iterate all entities across all types' call. The plan glosses over this. Implementation must (a) when opts.types is given, validate via Meta.GetEntityDef and error on unknown ones; (b) when opts.types is omitted, enumerate all entity-type names from r.deps.Meta and call ListEntities once per type. Empty-Meta case should return an empty map.
severity: significant
resolution: 'PLAN-KK2SE Approach now specifies per-type iteration: when opts.types is given, validate each via Meta.GetEntityDef and loop; otherwise enumerate all entity-type names from r.deps.Meta and call Store.ListEntities once per type. Empty-Meta returns an empty map. AC9 test fixture spans multiple entity types.'
status: addressed
---

# Finding

The plan says `entity_refs` "iterates entities via `r.deps.Store` (same path as
`rela.list_entities`)." But `Store.ListEntities` (`internal/lua/runtime.go:494`)
requires a non-empty `Type` in `store.EntityQuery`. There is no "iterate all
entities across all types" call.

Implementation must:

1. **`opts.types` given:** loop over the provided type names, validate each
via `r.deps.Meta.GetEntityDef`, error on unknown ones.
2. **`opts.types` omitted:** enumerate all entity-type names from
`r.deps.Meta` (likely via the `EntityDefs` map or an exposed iterator) and call
`ListEntities` once per type.
3. **Empty-Meta case** (no entity types defined): return an empty map; do
not error.

Also affects test coverage: AC8 ("covers all entities") needs a fixture spanning
more than one entity type, otherwise the per-type loop is untested.

# Resolution

Update Approach with explicit per-type iteration. Update test plan to include a
multi-type fixture for AC8.
