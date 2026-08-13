---
id: TKT-UGYSC8
type: ticket
title: 'CalDAV: declarative caldav: config — symmetrical single-type mapping + fail-fast validation'
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Add the declarative `caldav:` block to `dataentryconfig.Config` describing how
rela entities project to `VTODO` and how an inbound `VTODO` maps back. Design in
**RES-1Y2EB5** (Axis E).

Declarative-first is deliberate: operators maintain it, and it gives rela
leverage for tooling (validation, config UI, introspection) that a Lua mapper is
opaque to.

### One collection = one entity type = one symmetrical mapping

**A CalDAV collection declares exactly one `entity_type`, and that single
declaration serves both directions.** Read projects the entity out; write maps
the VTODO back. There is no separate `create:` block and no `sources:` list.

This deliberately **diverges from `feeds:`**, and the divergence is justified by
different constraints:

- **ICS is one URL per feed and read-only.** `Feed.Sources` is the only
mechanism there for combining entity types into one calendar — it also stands in
for the OR the filter language lacks.
- **CalDAV is one account URL enumerating N collections.** Verified in the live
test: the client issues `PROPFIND Depth:1` over the home-set and discovers every
collection, so an operator who wants tasks *and* bugs declares two collections
and the user configures the account once and sees both lists. Multi-collection
is native, so `sources:` has no job left to do.

What the single-type rule buys, beyond a smaller schema: the mapping becomes
**bidirectional by construction**. With multiple sources the read mapping is a
union while the write mapping is one branch of it, so a `create:` block has to
exist purely to re-state which branch — an asymmetry that is a symptom of the
list, not a requirement. Removing it means the create-target *is* the
collection's type: nothing to derive, nothing to disambiguate, and an inbound
`PUT` already knows the type before it consults the alias table.

**Accepted trade:** an interleaved mixed-type list (tasks and bugs in one sorted
list, one toggle, one colour) is not expressible. Two collections give the same
*visibility* with separate colours and independent toggling — arguably the
better default. If interleaving is ever wanted it is an ADDITIVE change
(`sources:` alongside `entity_type:`), not a rework.

### Schema

```yaml
caldav:
  tasks:
    meta: { name: "rela Tasks", color: "#C2185B" }
    component: vtodo            # VTODO-only collection (Apple requires this)
    entity_type: task
    where: ["status != done"]
    due: due
    summary: title
    description: notes
    completion:                 # the three-property completion event, one block
      status_property: status
      completed_value: done
      pending_value: todo
      completed_at: completed_at   # optional; receives COMPLETED
    defaults: { status: todo }  # applied to an inbound create
    on_delete: { set: { status: cancelled } }
  bugs:
    component: vtodo
    entity_type: bug
    where: ["status != closed"]
    due: target_date
    summary: title
    completion: { status_property: status, completed_value: closed, pending_value: open }
```

The `completion:` block maps `STATUS` / `COMPLETED` / `PERCENT-COMPLETE` as
**one logical event** in both directions, not three independent property
mappings — Apple writes all three together, and RFC 4791 §7.8.9 filters on
`COMPLETED` while UIs read `STATUS`, so a half-set state reads as done in one
client and pending in another. `calfeed.Todo.Complete` already enforces this on
the render side (TKT-SNBQX0).

### Deliverables

1. `CalDAVCollection` / `CalDAVCompletion` structs on `dataentryconfig.Config`.
Flat — no nested source list, no create block.
2. `validate_caldav.go` following `validate_feeds.go`: flat linear checks,
`[]string` messages prefixed with the collection name so an author can pinpoint
the YAML node, early-return once `entity_type` fails to resolve.
3. A Lua escape hatch, **mutually exclusive** with the declarative fields —
validated with the two-arm boolean switch `validateDocuments` uses for
Command-vs-Script (validate.go:1487).

### Validation rules

- `entity_type` resolves in the metamodel; every named property exists on it
with a compatible type.
- `completion.completed_value` / `pending_value` are members of the status
property's enum.
- **The entity type is constructible from `SUMMARY` alone** — every required
property is either mapped, has a `defaults:` literal, or has a template default.
(Verified: an Apple-created todo carries only `SUMMARY` + `STATUS` +
timestamps.)
- `component: vtodo` rejects VEVENT-shaped mappings and vice versa.

### The DEC-HWZHA departure (call out in review)

Required-property violations are **soft** today (`partitionValidationErrors`,
entitymanager/validation.go:22) — a write succeeds with a warning. So an
unsatisfiable mapping would not fail at write time; it would **silently create
warning-carrying invalid entities**. The constructible-from-`SUMMARY` check is
therefore the first place in the codebase where required-ness is fatal.
Intentional — document it as a deliberate departure rather than letting it look
like an inconsistency.

### Acceptance criteria

1. A valid `caldav:` block loads; each invalid case above produces a specific,
node-identifying error at startup, all collected in one pass.
2. `ValidateConfig` fails startup on an invalid block (matching `feeds:`).
3. Declarative and Lua mapper are mutually exclusive, with distinct messages for
"both set" and "neither set".
4. An entity type whose required properties cannot be satisfied from a bare
`SUMMARY` is rejected **at config load**, naming the unsatisfiable property.
5. Multiple collections over different entity types coexist in one config.
6. Table-driven tests in the `validate_feeds_test.go` style.

### Known limitation to document

`rebuildState` (watcher.go:286-300) **never calls `ValidateConfig`** —
hot-reload catches only YAML syntax errors. So these guarantees hold at
**startup only**. Either accept it and document it, or re-validate the CalDAV
slice on reload (small, well-scoped — decide in planning).

### Out of scope

- Executing the mapping (protocol ticket).
- The alias service.
- Interleaved mixed-type collections (additive later if ever wanted).
