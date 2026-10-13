---
id: TKT-RRKUNE
type: ticket
title: 'Auditor-grade dry-run report for rela migrate data: value breakdown, patch export, validation delta'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

`rela migrate data` (dry run) prints one line per step: `lua  taak ←
migrations/x.lua  would change 151 record(s)`, plus a single before/after count
of entities that fail property validation. That is enough for the operator, but
not for an auditor who has to approve a migration that changes compliance data.
Atlas's `taak.soort` migration (see TKT-ZO6GRZ) is the motivating case: a CISO
review asked what each task would become, and the answer had to be assembled by
hand.

## Proposal

1. **Per-step value breakdown.** For each step, count the new values per
changed property, for example `soort: compliance 79, corrective_action 28,
general 44`. Applies to every step kind that sets values (`lua`, `map_values`,
`convert`, `rename_property`, ...), not only `lua`.
2. **Per-entity patch export.** An option such as
`rela migrate data --report=patches.json` (or `.csv`) that writes one row per
changed entity per step: migration file, step, entity id, face, type, property,
old value, new value. Meant to be attached to the PR that adds the migration. It
must name values only for properties the operator could read anyway; the CLI
runs with operator trust, so no ACL filtering, but the file should say so in its
header.
3. **Validation delta per entity.** Replace the single before/after count with
the lists of entities that become newly invalid and newly valid, with the
failing rule for the newly invalid ones.

## Related

- BUG-LWVJPX (backlog) covers one narrow part of the same gap: `migrate_face`
reports entity counts only, not the edges it carried. That bug stays a separate,
smaller fix. This report should include the edge counts it adds, hence the
depends-on link.
- TKT-ZO6GRZ adds graph reads to `lua:` steps, which made relation-based
classifications possible and therefore made this report necessary.

## Open points

- A dry run writes nothing, so every step reads the pre-run store. A step
that depends on an earlier step's output reports what it would do against the
old data. The report should say so per file, or the runner needs an overlay so
later steps see earlier dry-run patches. Decide which.
- Memory: holding every patch for a large tenant. Stream the export to the
file instead of keeping it in `StepResult`.
