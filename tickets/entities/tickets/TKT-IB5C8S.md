---
id: TKT-IB5C8S
type: ticket
title: 'Meta-schema: commit the schema-that-describes-schemas as a reviewed artifact'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

The schema-as-graph spike hand-wrote a meta-schema (`schema.schema.yaml`)
describing what a rela schema *is* — entity types, relation types, properties,
custom types, plus the data-entry surfaces (forms, lists, kanbans, views,
fields, navigation). It worked, but it was a spike artifact with conventions
chosen ad hoc.

This ticket promotes it to a **reviewed, committed design artifact**. It is the
spec that the projector (next ticket) implements, so its conventions need
deciding once, deliberately.

## Scope

Produce a committed meta-schema covering the projected model, with these
conventions settled:

**1. Reserved-word collisions.** `type` is a reserved property name — a
`property` entity cannot have a `type` property (the loader rejects it). The
spike used `prop_type`. Meta-schemas collide with reserved words exactly where
they most want them (`id`, `type`). Decide the naming convention.

**2. ID namespacing.** Entity IDs are globally unique, not per-type. In the
spike `de-view/concept.md` was **silently shadowed** by `entity-type/concept.md`
and the Views list showed `0` with no error. Entity types and views routinely
share names (`concept`, `feature`). Decide the per-construct namespacing scheme
(spike used `view-`, `form-`, `list-`, `kanban-` prefixes).

**3. Comments as projected data.** Rather than treating YAML comments as
something to *preserve* around edits, project them as data — a property on the
owning entity, or their own entity type. This makes them editable from the SPA
and makes deletion/reordering carry them along by construction, instead of
relying on `yaml.v3`'s positional `HeadComment`/`LineComment`/`FootComment`
attachment.

**4. Relation set.** Structural: `has-property` (ordered — reproduces
`PropertyOrder` via `orderable: outgoing`), `from-type`/`to-type`, `uses-type`,
`has-field`, `validates`/`automates`. Cross-file: `renders-type`,
`binds-property`, `binds-relation`, `groups-by-type`,
`create-form`/`edit-form`/`detail-view`.

## Open questions (resolve when work starts)

- **Is a projected comment a property on the owning entity, or its own entity?** A property is simpler. A separate entity handles free-floating comments and section banners (`# ===== Architecture =====`) that belong to no single key — and `tickets/schema.yaml` has many of those. Likely both: an owned property for key-attached comments, an entity for standalone banners.
- **Do comments need position/ordering data** to round-trip faithfully, or is "attached to key X" sufficient?
- **Does the meta-schema live in-tree as a fixture, or is it generated** from the Go structs alongside the projector? Generating avoids drift but makes it harder to review.
- **Should `default_sort` project as ordered satellite entities** or stay opaque? It is an ordered list of specs on 18 entity types.

## Context

Spike findings: `.ignored/schemaspike/FINDINGS.md` §2 (model), §5.2 (ID
collision), §5.3 (reserved words).
