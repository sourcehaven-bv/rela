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

## The shape projection

Two separate questions run this system, and it helps to keep them apart:

- **Which migrations have run?** Answered by NAME, from a list the store
  keeps. That is what decides which files execute.
- **What shape does the stored data conform to?** Answered by the **shape
  projection**, a description of the data-relevant slice of the schema. That
  is what the compatibility gate classifies against and what `rela migrate
  gen` diffs to draft a migration.

The projection has a content hash, used as a cheap "did anything change?"
check. It is not an identity a migration is addressed by.

The projection covers a content hash of the
data-shape-relevant slice of the metamodel — entity properties (type,
required, list, format, values, default, computed expression), named enum value lists, and
relation types (endpoints, cardinality, symmetry, content flag, relation
properties). Everything else — labels, descriptions, colors, views, forms,
automations, validations, id prefixes — is excluded, so cosmetic edits never
demand a migration.

Each store records both answers — the applied migration names and the
projection its content conforms to — in a **migration record**. Where that
record lives is chosen per backend, because it describes the DATA and so has to
travel with it:

| Backend | Location | Why |
|---|---|---|
| filesystem | `migrations/applied.json`, **committed** | The entities and `migrations/` are tracked by git; a record under the gitignored `.rela/` would be missing from a fresh clone |
| PostgreSQL | `migration_state` in the tenant's schema | Each tenant tracks its own shape, so tenants at different points migrate independently |
| SQLite | `migration_state` in `rela.db` | A shipped single file must carry it, or the receiving copy replays every migration |

`migrations/applied.json` being committed is deliberate, and it is what Rails
does with `db/schema.rb` and EF Core with its model snapshot. It costs a
generated file in git that conflicts when two branches each add a migration —
and that conflict is the point. Git surfaces the collision so a human decides
the order, rather than both branches merging cleanly into an undefined one.

## The compatibility gate

At every process start (server and project CLI commands alike), the gate
compares the recorded projection against the live schema and classifies each
difference:

| Tier | Examples | What happens |
|---|---|---|
| **additive** | new entity/relation type, new optional property, new enum value, loosened cardinality, default changes | adopted silently |
| **drift** | deleted property, deleted entity/relation type, deleted enum value, new *required* or computed property, changed computed expression | adopted, with a logged notice per delta |
| **needs-migration** | property type/format change, `list` flip, enum value replacement, endpoint/cardinality narrowing, symmetry flip | **not** adopted — the gate warns and points at `rela migrate gen` |

**Who writes.** The gate always *classifies*; only the `rela migrate`
commands *record* what it classified. A running server never writes the
migration record — on the filesystem backend that record is a git-tracked
file, and a server dirtying an operator's working tree at boot (or needing a
writable project directory on a deploy box) would be wrong. It also means
several server processes starting at once have nothing to race over. The cost
is that a server can serve with an unrecorded *additive* change, which by
definition cannot invalidate stored content; the next `rela migrate` records
it.

**A store with no record.** With no migrations in the project, the live shape
is adopted as the baseline silently — an existing project joins the system
without ceremony.

With migrations present, **every command refuses to guess**, including
`rela migrate data` itself. Those files may be exactly the ones this store still
needs — or exactly the ones it has already run, with its record lost. Nothing
distinguishes the two from the outside, and both wrong answers lose data:
baselining marks pending migrations applied forever, while running them replays
transforms over already-migrated content.

Resolve it explicitly:

```bash
rela migrate baseline --apply   # the content already matches: record them as applied
```

A fresh clone of a project whose `schema.yaml` moved ahead of its committed
entities is the everyday way to reach this — as is an `applied.json` that was
gitignored by accident.

**Upgrading from the pre-`applied.json` scheme** lands here too. The old record
named migrations `0001-…`, which is not a valid name under the timestamp scheme,
so it cannot be carried across: rename the files in `migrations/`, then
`rela migrate baseline --apply` to record the set. The upgrade refuses rather
than adopting a partial list, because a partially-converted record reads as
"nothing has run" and would replay the whole chain.

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

Migrations are YAML files you commit under `migrations/`, named with a
**timestamp prefix** (`20260919143022-rename-status.yaml`). Names sort
lexicographically, which for a timestamp is chronologically, so the file order
is the run order.

A timestamp rather than a sequence number because sequence numbers collide
silently across concurrent branches: two pull requests cut from the same commit
both pick the same next index, and since nothing else about the filenames
differs, git merges both cleanly into an undefined order.

Each file embeds the schema shape before and after it, so it is self-contained:

```yaml
description: status rework
steps:
  - rename_property: {entity: task, from: status, to: state}
  - map_values:
      entity: task
      property: state
      mapping: {open: todo, wip: doing}
  - convert: {entity: task, property: due, to_type: date, from_format: "01/02/2006"}
from_projection: { ... }   # the shape before this migration
to_projection: { ... }     # the shape after it
```

The projections are not decoration. Step targets are validated against the
shapes **this file** spans — `rename_property{from: status, to: state}` is
well-formed only where `status` exists in the from-shape — and by the time a
later migration has run, the live schema no longer contains it. They are also
what lets a file be refused when it spans a change its steps do not answer (see
`migrate_face` below).

### Data-only migrations

A migration does not have to change the schema. A backfill, a de-duplication,
or a correction of values an old bug wrote is an ordinary migration file whose
two projections are the same:

```yaml
description: backfill owner on tasks that have none
steps:
  - set_default: {entity: task, property: owner, value: unassigned}
from_projection: { ... }   # identical to to_projection
to_projection: { ... }
```

It runs because its name is not in the applied list. `rela migrate gen` will
not draft one — there is no schema diff to draft from — so write it by hand.

### Steps

Every step is **idempotent**: re-running a migration after a crash is the
recovery mechanism, so a step that finds nothing left to do does nothing.

| Step | Effect |
|---|---|
| `rename_property: {entity, from, to}` | moves the value to the new key (only where the old key exists) |
| `rename_entity_type: {from, to}` | rewrites `type:` on every entity of the old type (IDs are unchanged) |
| `rename_relation_type: {from, to}` | recreates each relation under the new type, then deletes the old (relation history starts a new lifetime) |
| `rename_face: {entity, from, to}` | moves every row stored at one content state to another (IDs are unchanged) |
| `migrate_face: {entity, property, mapping}` | moves existing rows onto the face they belong to when a type gains its first faces; **required** in any file spanning that change |
| `map_values: {entity, property, mapping}` | remaps enum values (scalar and list properties); unmapped values are left and reported |
| `set_default: {entity, property, value, only_missing}` | backfills a value (`only_missing` defaults to true) |
| `recompute_computed: {entity}` | recomputes all materialized computed properties for an entity type in dependency order |
| `convert: {entity, property, to_type, from_format?, to_format?}` | coerces values to `string`/`integer`/`boolean`/`date`/`datetime`, restructuring scalar↔list to match the schema; unconvertible values are **left in place** and reported |
| `drop_property: {entity, property}` | deletes orphaned values (only for properties the new schema no longer declares) |
| `drop_entities: {type}` / `drop_relations: {type}` | deletes records of a type the new schema no longer declares |
| `lua: {entity, script}` | the escape hatch — see below |

Step targets are validated against the embedded projections when the file is
parsed: a typo'd entity type or property is an error, never a silent no-op.
Migration names are validated too — a name must be
`<14-digit timestamp>-<lowercase-slug>.yaml` — because the applied list records
names and compares them against directory entries, and a case-folding
filesystem would otherwise make one file look like two entries.
Deletes are first-class steps — putting one in a reviewed migration file *is*
the operator consent — but the generator only ever emits them commented out.
Adding or changing a computed property emits one active
`recompute_computed` step per affected entity type. The step refreshes the
whole graph, so dependent computed properties cannot retain stale values.

### Renaming a content state

`rename_face` moves rows between coordinates. A face is not part of the entity
id — it is a separate field — so no id is rewritten and relations keep their
endpoints, exactly as with `rename_entity_type`.

A face's declared name is its stored coordinate, so a rename is always a
named-to-named move and there are no special cases. A rename whose source and
destination are the same name is a no-op and the step does nothing.

A rename onto an occupied coordinate is refused: the entity has content at
both, so this is a *merge* rather than a rename, and moving would destroy one
side. Decide which content wins and express it as a drop plus a rename.

### Adding or removing faces on a type that holds data

Gaining or losing `faces:` needs a migration, and the classifier says so:
`faces_introduced` and `faces_removed` are both needs-migration findings rather
than drift.

The reason is that a type declaring no faces keeps its single state at the
**zero coordinate**, while a type declaring `faces:` keeps every state under a
face name and nothing at the zero coordinate. Adding faces to a populated type
therefore leaves every existing row at a coordinate that names no declared
face. Nothing looks broken — no row moved and no value changed — which is
exactly why the store will not adopt the shape on its own: only you can say
which face the existing content became.

`rename_face` cannot express this move. It requires a declared face name on
both sides, and the zero coordinate is not one, so a project crossing this
boundary with data in it needs the rows rewritten out of band before the new
schema is adopted. Plan the change on an empty type where you can, and treat a
populated one as a data-export-and-reimport rather than a step in a migration
file.

Removing faces is the mirror: rows sitting at named faces belong to no declared
face afterwards, and the type's single state is a coordinate none of them
occupies.

Two consequences are worth checking at the same time, because neither produces
a load error:

- **Write grants in `acl.yaml` stop matching.** A bare `update: [policy]`
  addresses the zero coordinate, so once `policy` declares faces the grant
  reaches nothing. Rewrite it to name each face — `update: [policy@draft]` —
  and run `rela acl audit`, which reports the bare form as
  `B12-bare-grant-on-faced-type`.
- **Creates must name a face.** A `POST` that omits one is refused with
  `face_required` once the type is faced. See the
  [Content States guide](content-states.md) for the request shape.

### Adopting content states on data you already have

Giving a type its first faces needs a migration, and the reason is that nothing
visible happens. A type with no faces stores its single state at the zero
coordinate, which names no face; declaring faces leaves every existing row
sitting there. No row moves, no value changes, and the rows now belong to no
declared face — which is why the store will not adopt the shape on its own.

`migrate_face` moves them:

```yaml
- migrate_face:
    entity: article
    property: status
    mapping:
      draft:     draft
      active:    published
      withdrawn: published
```

Each value of the keying property names the face its rows move to. The move is
real: the row is created at the new coordinate and the zero-coordinate row is
removed.

**The mapping must cover every value of the property.** That is the safety
property, not a formality — a value you leave out keeps its rows at the zero
coordinate, where they name no face, and once the keying property is dropped
(often in the same migration) nothing records what they were. Requiring every
value to name a face turns "I did not think about `withdrawn`" into a parse
error instead of a silent loss.

Two more rules:

- **Put it before any `drop_property` of the property it reads.** The wrong
  order is refused at parse time, since the step would have no values to key on.
- **Rows whose value is unset, or outside the declared set, stay where they
  are** and are reported. They are not given a guessed face.

**A file that spans this schema change and does not migrate the rows is
rejected.** That is the point of the step: before it existed, such a file
parsed, applied, advanced the marker and reported the schema in sync while every
row was left stranded. `rela migrate gen` therefore drafts a real `migrate_face`
step with every value pre-listed against `CHANGEME`, and `CHANGEME` is not a
face — so an unedited draft will not apply either. You have to say where the
rows go.

Faced → flat (`faces_removed`) is the mirror case and is not covered: rows at
named faces would have to move back, and deciding which one wins when several
hold content is a merge rather than a move.

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
edit schema.yaml              # the incompatible change
rela migrate gen              # drafts migrations/<timestamp>-<slug>.yaml
$EDITOR migrations/2026*      # review: confirm GUESSes, fill TODOs
rela migrate data             # dry-run: per-step counts + validation delta
rela migrate data --apply     # execute
git add schema.yaml migrations/ && git commit
```

Commit `migrations/applied.json` along with the migration: on the filesystem
backend it is how a clone knows what has already run.

`gen` diffs the recorded projection against the live schema and emits
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

`data` runs every migration whose name is not in the applied list, in file-name
order. A store that is several versions behind catches up in one run, and a
tenant with its own applied list catches up independently of its siblings. The
record advances after each file completes — a crash mid-run is recovered by
re-running.

**The applied list is the only thing preventing a double-apply**, which makes
step idempotency load-bearing rather than merely advisable. Every step must be
safe to re-run: re-running after a crash is the documented recovery path, and a
record restored from a backup will replay whatever it has forgotten. The
declarative steps are idempotent by construction; a `lua:` step is idempotent
only if you wrote it that way.

Execution writes raw batches to the store (bypassing per-entity validation,
automations, and ACL — the trust boundary is your shell, exactly like
`rela db migrate`). Each applied file emits one audit record with names and
counts, never content. On PostgreSQL, migrated content appears in
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
