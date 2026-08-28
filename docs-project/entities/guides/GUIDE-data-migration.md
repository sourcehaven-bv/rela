---
audience: advanced
id: GUIDE-data-migration
order: 25
status: published
summary: Detect schema shape changes and migrate stored content with generated, reviewable migrations
title: Data Migration
type: guide
---

When `schema.yaml` changes shape — a property renamed, a type changed, enum
values remapped — the entities and relations already stored no longer match
the schema. The data-migration system detects this, adopts harmless changes
automatically, and gives you generated, reviewable migration files for the
changes that genuinely require transforming stored content.

This is distinct from `rela migrate` (which upgrades the *syntax* of config
files like `schema.yaml` itself) and from `rela db migrate` (PostgreSQL DDL).
Both of those run before anything described here.

## The shape hash

The unit of identity is the **shape hash**: a content hash of the
data-shape-relevant slice of the metamodel — entity properties (type,
required, list, format, values, default, computed expression), named enum value lists, and
relation types (endpoints, cardinality, symmetry, content flag, relation
properties). Everything else — labels, descriptions, colors, views, forms,
automations, validations, id prefixes — is excluded, so cosmetic edits never
demand a migration.

Each store records the shape its data conforms to in a **marker**
(`migration/state.json` in the store's state KV: under `.rela/` on the
filesystem backend, in the `state_kv` table on PostgreSQL — so with
schema-per-tenant every tenant has its own marker and migrates
independently).

## The compatibility gate

At every process start (server and project CLI commands alike), the gate
compares the marker against the live schema and classifies each difference:

| Tier | Examples | What happens |
|---|---|---|
| **additive** | new entity/relation type, new optional property, new enum value, loosened cardinality, default changes | adopted silently |
| **drift** | deleted property, deleted entity/relation type, deleted enum value, new *required* or computed property, changed computed expression | adopted, with a logged notice per delta |
| **needs-migration** | property type/format change, `list` flip, enum value replacement, endpoint/cardinality narrowing, symmetry flip | **not** adopted — the gate warns and points at `rela migrate gen` |

A store with no marker adopts the current shape as its baseline silently —
existing projects join the system without ceremony.

While a needs-migration change is pending, writes keep today's behavior
(soft validation warnings); nothing blocks. Run `rela migrate status` to see
where a store stands.

### The rename blind spot

A diff cannot distinguish *rename `status` → `state`* from *delete `status`,
add `state`*. Both classify as drift and auto-adopt, but the gate flags the
pair loudly: if it **was** a rename, generate and run a migration before the
orphaned values are garbage-collected. The GC grace period (below) is the
safety net — a missed rename costs eventual, recoverable loss, never
immediate loss.

## Migration files

Migrations are YAML files you commit under `migrations/`, named with an
ordering prefix (`0001-rename-status.yaml`). Each is an edge from one shape
hash to another, with both projections embedded so the file is fully
self-contained:

```yaml
from: <shape hash the data currently conforms to>
to: <shape hash after this migration>
description: status rework
steps:
  - rename_property: {entity: task, from: status, to: state}
  - map_values:
      entity: task
      property: state
      mapping: {open: todo, wip: doing}
  - convert: {entity: task, property: due, to_type: date, from_format: "01/02/2006"}
from_projection: { ... }   # embedded, integrity-checked against `from`
to_projection: { ... }
```

### Steps

Every step is **idempotent**: re-running a migration after a crash is the
recovery mechanism, so a step that finds nothing left to do does nothing.

| Step | Effect |
|---|---|
| `rename_property: {entity, from, to}` | moves the value to the new key (only where the old key exists) |
| `rename_entity_type: {from, to}` | rewrites `type:` on every entity of the old type (IDs are unchanged) |
| `rename_relation_type: {from, to}` | recreates each relation under the new type, then deletes the old (relation history starts a new lifetime) |
| `map_values: {entity, property, mapping}` | remaps enum values (scalar and list properties); unmapped values are left and reported |
| `set_default: {entity, property, value, only_missing}` | backfills a value (`only_missing` defaults to true) |
| `recompute_computed: {entity}` | recomputes all materialized computed properties for an entity type in dependency order |
| `convert: {entity, property, to_type, from_format?, to_format?}` | coerces values to `string`/`integer`/`boolean`/`date`/`datetime`, restructuring scalar↔list to match the schema; unconvertible values are **left in place** and reported |
| `drop_property: {entity, property}` | deletes orphaned values (only for properties the new schema no longer declares) |
| `drop_entities: {type}` / `drop_relations: {type}` | deletes records of a type the new schema no longer declares |
| `lua: {entity, script}` | the escape hatch — see below |

Step targets are validated against the embedded projections when the file is
parsed: a typo'd entity type or property is an error, never a silent no-op.
Deletes are first-class steps — putting one in a reviewed migration file *is*
the operator consent — but the generator only ever emits them commented out.
Adding or changing a computed property emits one active
`recompute_computed` step per affected entity type. The step refreshes the
whole graph, so dependent computed properties cannot retain stale values.

### The Lua escape hatch

For transforms no declarative step covers (splitting one property into two,
cross-record derivations), a `lua:` step runs an operator-authored **pure transform**:

```lua
-- migrations/0002-split-name.lua
function migrate(entity)
  -- entity = { id, type, content, properties = {...} }
  local name = entity.properties.name
  if name == nil then return nil end          -- nil = leave unchanged
  local first, last = string.match(name, "^(%S+)%s+(.*)$")
  return {
    properties = { first_name = first, last_name = last },
    unset = { "name" },                        -- properties to remove
    -- content = "...",                        -- optionally replace the body
  }
end
```

The script never writes anything itself — the migration runner applies the
returned patch with raw store access. That is deliberate: migration input is
by definition invalid under the new schema, so the normal write path
(validation, automations, state machines) must not run. The sandbox has no
`io`, `os`, or file access. Use `entity: "*"` (quoted) to transform every
entity, including entities whose type the new schema no longer knows.
Scripts must be idempotent, like every other step.

## Workflow

```text
edit schema.yaml            # the incompatible change
rela migrate gen            # drafts migrations/000N-schema-change.yaml
$EDITOR migrations/000N-*   # review: confirm GUESSes, fill TODOs
rela migrate data           # dry-run: per-step counts + validation delta
rela migrate data --apply   # execute
git add schema.yaml migrations/ && git commit
```

`gen` diffs the marker's stored projection against the live schema and emits
best guesses: same-shaped remove+add pairs become `rename_property` /
`rename_entity_type` steps marked `# GUESS`, enum replacements become
`map_values` stubs marked `# TODO`, type changes become `convert` steps, and
deletions appear only as commented-out cleanups. **The review is the safety
mechanism** — never apply a draft unread.

Applies are serialized by a **migration lock**: `rela migrate data --apply`,
`rela migrate gc --apply`/`--scan`, and the server's GC sweep all take a
per-store lock before writing, so two concurrent runs cannot interleave — the
second fails fast with "another migration or GC run is active" (the sweep
just skips its cycle, and the startup gate skips persisting an adoption
until the holder finishes). On PostgreSQL the lock is a schema-scoped
advisory lock, so tenants sharing a database never block each other; on the
filesystem backend it is a lock file under `.rela/` (single machine, with
stale-lock detection after a crash). Dry-runs never take the lock.

One caveat on crash recovery: staleness is judged by whether the recorded
process id is still alive on this machine, never by age (a long migration
is not a crash). If a crashed run's pid has been recycled by an unrelated
process, the lock stays honored — the remedy is simply removing
`.rela/migration.lock` by hand once you have confirmed no migration is
running.

`data` resolves the chain from the store's current hash to the live schema.
Migrations run in file-name order; compatible gaps between them (additive
changes that were adopted without a migration) are bridged automatically, so
a tenant that is several versions behind catches up in one run. Already
applied files (recorded in the marker) are skipped. The marker advances
after each file completes — a crash mid-run is recovered by re-running.

Execution writes raw batches to the store (bypassing per-entity validation,
automations, and ACL — the trust boundary is your shell, exactly like
`rela db migrate`). Each applied file emits one audit record with names,
hashes and counts, never content. On PostgreSQL, migrated content appears in
the version history attributed to the operator with the `data-migration`
tool, and destructive steps capture pre-delete snapshots synchronously.

## Garbage collection

Deleted-from-the-schema data is **not** deleted from the store. It sits
orphaned (invisible to validation, preserved by partial writes) and is
recorded in a drift ledger with a first-seen timestamp. A periodic GC sweep
removes it only after a **grace period** (default 30 days):

- The sweep never runs while the gate reports needs-migration — a pending
  migration may be about to transform exactly that data.
- If the schema re-declares a property or type before the deadline, the
  ledger entry is dropped and the data survives.
- Deletions are audited (`data-gc` records) and, on PostgreSQL, captured
  into version history first.

Controls: `RELA_DATA_GC=off` disables the sweep; `RELA_DATA_GC_INTERVAL`
(default `1h`) and `RELA_DATA_GC_GRACE` (default `720h`) tune it. Run it
manually with `rela migrate gc` (dry-run) / `rela migrate gc --apply`;
`--scan` additionally sweeps the store for legacy orphans that predate the
ledger. To delete orphaned data *immediately*, put an explicit `drop_*` step
in a migration instead.

## Two hashes, on purpose

PostgreSQL version history content-addresses a *render* projection of the
schema (how a historical version displays), while migration uses the *shape*
projection (whether stored data fits). They churn on different edits by
design — a new display label changes neither; a relation-property change
moves only the shape hash. Don't try to unify them.
