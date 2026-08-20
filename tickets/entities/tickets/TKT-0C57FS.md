---
id: TKT-0C57FS
type: ticket
title: 'Data migration system: shape hash, compatibility classifier, declarative migrations, GC sweep'
kind: enhancement
priority: medium
effort: l
status: done
---

## Problem

Today schema drift is mostly silent: property renames/type changes only produce
soft validation warnings on write (DEC-HWZHA,
`internal/metamodel/validation.go:113`); an entity-type rename hard-422s every
write; nothing blocks or guides at startup. There is no way to express "the data
must change because the schema changed". The existing `internal/migration`
package rewrites *config* YAML only (detect-idempotent, no versioning, no store
access) and does not fit.

## Design

### 1. Shape identity: `metamodel.ShapeProjection`

New projection alongside `RenderProjection`
(`internal/metamodel/projection.go`), same hasher discipline (length-prefixed,
key-sorted SHA-256), different contents:

- **In**: entity types → properties (type, required, list, format, inline values, default), custom types (enum value lists), relation types (from/to, cardinality, relation properties).
- **Out**: views, forms, automations, validations, colors, descriptions, labels, `DisplayProperty`, `PropertyOrder`, ACL, **id prefixes** (see amendment A5 — no v1 step could migrate ids, so a prefix change must not create an unresolvable needs-migration state).

`shapeHash = ShapeProjection().Hash()` is the migration version. Do NOT modify
`RenderProjection` — its stability contract is load-bearing for pgstore version
rendering (`schema_versions`). Two hashes coexist by design; document the
distinction.

### 2. Compatibility classifier

`metamodel.CompareShapes(from, to) → Report`, three tiers:

| Tier | Deltas | Startup behavior |
|---|---|---|
| additive | new entity/relation type, new optional property, new enum value, widenings, **default-value changes** (affect future creates only — A7) | silently adopt new hash |
| drift | deleted property, deleted entity/relation type, deleted enum value, new required property, delete+add pairs | adopt, log a notice per delta (incl. GC deadline) |
| needs-migration | property type/format change, `list` flips, enum value replacement, narrowings | warn/banner, point at `rela migrate gen` / `rela migrate data` |

Notes grounded in current behavior:

- Deleted property values persist indefinitely (PatchEntity deliberately preserves unnamed properties — `TestPatchEntity_PreservesUnnamedProperties`); harmless but permanent → GC territory.
- Deleted entity type: entities become `unknown_type` → read-only (hard 422 on write). Drift notice must say so.
- **Rename ambiguity**: a diff cannot distinguish `delete X + add Y` from `rename X→Y`. Classifier flags same-type removed+added pairs prominently ("if this is a rename, run rela migrate gen") but still auto-adopts. Mitigated by the GC grace period + recoverability (entity_versions on pg, git on fs). Own this hole visibly; no classifier fixes it.

### 3. Migration files

Operator-authored, committed, in `migrations/` (e.g.
`migrations/0001-rename-status.yaml`):

```yaml
from: <shape hash the migration expects>
to:   <shape hash after the change>
from_projection: {...}   # embedded FROM ShapeProjection JSON — see A2
description: ...
steps:
  - rename_property: {entity: task, from: status, to: state}
  - map_values: {entity: task, property: state, mapping: {open: todo}}
  - set_default: {entity: task, property: priority, value: medium, only_missing: true}
  - rename_entity_type: {from: req, to: requirement}
  - convert: {entity: task, property: due, to_type: date}   # built-in coercions
  - drop_property: {entity: task, property: legacy}          # deletes are first-class, operator-reviewed
  - lua: {entity: task, script: migrations/0001-split.lua}
```

- **Embedded FROM projection (A2)**: chain resolution's free edges need `CompareShapes(current.projection, migration.from_projection)`, and plan-time step validation needs the FROM shape — hashes alone can't supply either, and no reliable historical-projection source exists (schema_versions is pg-only and a different projection type). `gen` embeds the FROM ShapeProjection JSON in the file; `from` hash must match its hash (integrity-checked at parse). Self-contained beats a sidecar.
- **Every step idempotent by construction** (rename fires only if old key present; map_values only maps old values; set_default fills gaps). This is the crash-recovery story: runs are NOT atomic (fs has no rollback; one giant Tx on pg is forbidden — stalls all writers). Recovery = re-run; marker written only after full success.
- **Lua escape hatch is a pure transform**: step receives the old entity table, returns a patch; the ENGINE applies it. No `lua.Mutator`/WriteDeps — a normal Lua write would go through entitymanager (validation rejects `unknown_type`, automations fire, state machines guard: all wrong mid-migration). Read-only `rela.*` env for lookups is fine. Documented as must-be-idempotent.
- **Resolution is a hash-chain walk with free edges**: migrations are edges you must take; compatibility is an edge you get free. A store at H1 (never migrated) can bridge H1→H2 via CompareShapes(additive/drift) then apply the migration from:H2. Keeps multi-tenant pg (tenants at different hashes) workable without no-op migrations.
- **`rename_entity_type` is a dedicated step, NOT a renametype delegation (A3)**: `internal/renametype` also rewrites schema.yaml, which has ALREADY changed by migration time — only its entity-rewrite logic is reference material. And a naive `UpdateEntity` with a changed type orphans the old file on fsstore (`fsstore/entity.go:263` writes the new-path file, never removes the old one). Fix: make type-change-on-update a store contract — fsstore's `updateEntity` relocates the file when `Type` differs; pin with a storetest conformance case (all backends). The step is then a plain UpdateEntity everywhere, plus template handling.

### 4. Generation: `rela migrate gen`

Diffs the marker's stored projection against live `schema.yaml` shape; emits a
draft:

- removed+added property, same type/format → `rename_property` annotated `# GUESS — confirm`
- enum value diff → `map_values` with `# TODO` stubs for removed values
- type change with known coercion → `convert`; without → skeleton `lua:` step
- deletions → commented-out `drop_*` (never emitted live; human uncomments)
- removed+added entity type of similar shape → `rename_entity_type` guess

Only needs-migration deltas produce steps; drift produces optional commented
cleanups (e.g. `set_default` backfill for new required property with default).
The emitted file embeds the FROM projection (A2).

### 5. Execution: `rela migrate data`

Order: config migrations (existing `rela migrate`) → shape comparison → data
migration chain.

- Dry-run by default (per-step affected counts + before/after `schema.ValidateEntityProperties` delta); `--apply` executes. Follows `normalize`/`history-purge` precedent.
- Writes are raw batched `store.UpdateEntity` (collect ids, small Tx batches on pg; per-file on fs), deliberately bypassing entitymanager. Sanctioned exception, operator-shell trust boundary like `db migrate`/`history-purge`; no ACL.
- **Synchronous version capture for destructive/rename steps (A1)**: on pg, delete and rename capture happens at the entitymanager boundary and the sweep CANNOT reconstruct it. Since the runner bypasses entitymanager, it must do its own capture: type-assert the optional `VersionWriter`/`RelationVersionWriter` capabilities (exactly as entitymanager's version_hook does) and record pre-delete/pre-rename snapshots for `drop_entities`/`drop_relations`/`rename_entity_type` — including cascade-deleted relations via `DeleteResult.DeletedRelations`. `drop_property`/value rewrites are updates, which the sweep captures normally. Stores without the capability (fs/mem): no-op — git is the recovery there.
- Attribution: `system:migration` principal + `store.WithAttribution` so pg stamps `last_edited_by_*` and the version sweep attributes new `entity_versions` rows correctly (migrated content SHOULD appear in history; content-hash dedup makes untouched entities free).
- Audit: one explicit record per migration run via `writeServices.Audit` (`internal/cli/cli_wiring.go:47` precedent) — migration name, from/to hashes, per-step counts; never content.
- Locking (pg): own advisory lock key; mutually exclusive with the version sweep (purge-vs-sweep reasoning).
- Change events fire naturally from raw writes (search reindex, SPA updates); batching bounds the burst.

### 6. Applied-state marker

`state.KV` key `migration/state.json`: `{shape_hash, projection (full JSON),
applied: [names], updated_at}`.

- Per-store for free: `.rela/` on fs, `state_kv` per schema on pg (per-tenant state).
- Full projection stored so gen diffs without git archaeology.
- Bootstrap: no marker → adopt current shape hash on first write-capable startup.
- `state.KV` has no CAS → all marker writes under the migration lock. Updated on every adoption so gen always diffs against what data actually conforms to.

### 7. Gate: startup AND metamodel reload (A4)

1. Hash match → done (zero cost).
2. Mismatch → CompareShapes(marker.projection, live): all-additive → adopt silently; drift → adopt + notices (once, not per request); needs-migration → do NOT adopt, warn with concrete deltas + fix command. Writes keep today's soft-warning behavior unless `strict` configured (warn-by-default; a hard gate would break every project with benign drift, consistent with DEC-HWZHA).
3. Adoption only in write-capable processes, under lock, idempotent.
4. **The gate re-evaluates on metamodel hot-reload**, not just boot: FSLoader re-loads schema.yaml on fsnotify events, so an operator editing the schema under a running server introduces drift mid-run. Gate subscribes to reloads, recomputes, publishes its verdict via `atomic.Pointer` (state-publish rule); the GC sweep reads the LATEST verdict each tick.

### 8. GC sweep

Default cleanup is a periodic sweep; migrations can delete immediately (operator
control), but drift normally ages out via GC.

- **Drift ledger** `migration/drift.json` in state.KV, **keyed by schema name** (`type` or `type.property`), populated from the classifier's drift report — NOT by scanning content (A6). Content is read only when counting (dry-run/notices) and applying deletions, with targeted queries on pg where possible. Each tick: add newcomers with first-seen timestamp, REMOVE entries no longer orphaned (schema re-added, or migration transformed). Only entries older than grace period (default ~30 days, configurable) are deleted.
- Grace period defuses the rename hole: missed rename costs eventual (recoverable) loss, not immediate; drift notice states the deadline.
- Placement: goroutine with server lifecycle (version-sweep pattern), own pg advisory lock, mutually exclusive with a running migration. Engine is backend-agnostic, shared by: server goroutine, `rela migrate gc --apply` (manual/immediate), scheduler task (headless fs projects).
- **Deletions capture versions synchronously** on pg (same A1 mechanism as migration delete steps).
- Attribution `system:gc-sweep`; one audit record per tick that deleted something (counts/names, never content). **The audit sink is a constructor dependency of the GC engine** (nil-rejected), injected at the appbuild wiring site — the same `audit.Audit` the entitymanager hook uses; `writeServices.Audit` is only the CLI's route to it (A8).
- **Ordering rule**: sweep never touches anything while the gate reports needs-migration deltas — skip the tick entirely (reads the latest gate verdict, per A4). Only adopted drift is eligible.
- Config: grace period + on/off, operator config; ON by default.

## Design review amendments (2026-08-20, RR-linked)

- **A1 (RR-DU4BUS)**: migration runner + GC engine perform synchronous version capture for deletes/renames via optional `VersionWriter`/`RelationVersionWriter` capabilities, incl. cascade-deleted relations; no-op on stores without the capability.
- **A2 (RR-5TYGFO)**: migration files embed the FROM ShapeProjection JSON; `from` hash integrity-checked against it at parse; enables free-edge resolution and plan-time step validation.
- **A3 (RR-FVCHUA)**: `rename_entity_type` is a dedicated step; renametype NOT delegated (it rewrites schema.yaml, already changed). Fix fsstore `updateEntity` to relocate the file on type change; pin type-change-on-update as a store contract via storetest.
- **A4 (RR-FURO8P)**: gate runs on metamodel hot-reload too; verdict published via atomic.Pointer; sweep reads latest verdict per tick.
- **A5 (RR-JPYXQ9)**: id prefixes EXCLUDED from v1 ShapeProjection (no step could migrate ids → would create unresolvable needs-migration). Prefix changes remain the existing config-migration concern; a future `reprefix_ids` step may re-add them.
- **A6 (RR-P64QYC)**: drift ledger keyed by schema names from the classifier report; no full content scan per tick.
- **A7 (RR-7IOBDB)**: default-value-only changes explicitly tiered additive.
- **A8 (RR-3AGN9Y)**: GC engine takes the audit sink as a nil-rejected constructor dep, wired in appbuild.

## Known plumbing constraints

- `requiresProject` (`internal/cli/kong.go:226`) matches only the first command token, and bare `rela migrate` deliberately runs WITHOUT services (repairing broken projects). Fix the matcher to full command path so `migrate data|gen|status|gc` get `writeServices` while bare `migrate` stays service-free.
- `internal/migration`'s `Migration` interface (`func(*yaml.Node) error`) does not fit; data migrations get their own type/runner (sibling package or same package, different type).
- Metamodel aliases (`aliasMap`) half-handle type renames on the read path: position as complements — alias keeps old references readable during transition, migration moves the data; `rename_entity_type` optionally drops the alias. Record as a decision.
- `schema_versions.hash` includes the `purge-tombstone` sentinel — don't assume every row is a real projection hash if reusing it for projection lookup.

## Acceptance criteria (sketch — refined in PLAN-OX2A9U)

1. Cosmetic schema edits (views, descriptions, ordering, automations, **defaults**, **id prefixes**) do not change the shape hash / do not demand migration.
2. Additive change: startup adopts silently; no migration required; marker updated.
3. Drift change: startup adopts with notice; deleted-property values survive until GC grace expires; delete+add pair produces the possible-rename warning.
4. Needs-migration change without migration: startup warns with concrete deltas; writes still work (soft warnings).
5. `rela migrate gen` emits correct guesses for rename/enum-remap/convert with GUESS/TODO annotations; deletions only commented out; FROM projection embedded and hash-consistent.
6. `rela migrate data` dry-run shows per-step counts and validation delta; `--apply` transforms all entities, is re-runnable after a mid-run crash, writes one audit record, updates the marker only on full success.
7. Chain resolution bridges compatible gaps: store at older hash with intervening adopted-only changes still reaches the live hash.
8. Lua step: pure transform applied by the engine; a transform touching an entity of `unknown_type` still applies (no entitymanager validation).
9. GC sweep: respects grace period, skips ticks while needs-migration deltas pending (latest reload-aware verdict), removes ledger entries when schema re-adds a property, audits deletions, captures pre-delete versions on pg, works on fs and pg.
10. Multi-tenant pg: two schemas at different hashes migrate independently; locks scoped per schema.
11. All three backends; `rename_entity_type` leaves no orphan file on fsstore (storetest-pinned); destructive steps produce version snapshots on pg.
12. Metamodel hot-reload re-runs the gate; a mid-run incompatible edit surfaces notices/warnings without restart.

## Out of scope (v1)

- `split_property`/`merge_property` declarative ops (Lua covers them).
- Automatic backfill of new required properties (gen offers commented `set_default`).
- Blocking/strict gate as default (config option only).
- GC of stale enum *values* inside data (that is `map_values` territory, not orphan cleanup).
- id-prefix migration (`reprefix_ids`) — prefixes excluded from the shape hash in v1 (A5).
