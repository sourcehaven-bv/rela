---
id: TKT-MW28U5
type: ticket
title: 'Hierarchical Gantt view for data-entry (gantts: config, recursive roll-up, drill-down)'
kind: enhancement
priority: medium
effort: xl
status: review
---

## Summary

Add a `gantts:` view type to `data-entry.yaml`, rendering entities against a
time axis nested by a containment relation. The defining constraint is
**recursive self-referential containment**: a `project` may contain sub-projects
to unbounded depth, so depth is navigated (drill-down) rather than displayed
(indentation).

An interactive prototype exists at `mockups/gantt-drilldown.html` — a single
self-contained file, no dependencies. It was driven in a browser and the
findings below come from that, not from speculation.

## Config shape

```yaml
gantts:
  delivery:
    title: "Delivery plan"
    hierarchy: [contains, has-epic, has-ticket]   # traversed as ONE set
    multi_parent: first        # first | duplicate | error
    on_cycle: error            # error | prune
    default_depth: 2
    sources:
      project: { start: planned_start,   end: planned_end, committed: target_date }
      epic:    { start: start,           end: end,         committed: ~ }
      ticket:  { start: scheduled_start, end: due,         committed: ~ }
```

Three points the prototype settled:

- **`hierarchy` is a list, not a single relation name.** This handles
homogeneous (`[contains]`) and mixed hierarchies with one mechanism; the
renderer follows any edge in the set downward and does not care which.
- **Date roles are mapped per entity type.** Real schemas do not agree on
property names — a ticket has `due`, an epic has `end`. All three roles are
independently optional: no `start`/`end` yields a pure roll-up bar; a
`committed` alone yields a milestone target.
- **`multi_parent` and `on_cycle` are explicit policies**, because the graph
permits both conditions and neither has a safe silent default.

## Planned vs. rolled-up spans

Two spans per node, deliberately **not merged**:

- `planned` — what the entity declares via its own mapped properties
- `rolled` — the envelope of descendants (recursive post-order fold)

The rendered bar spans their union; the planned window is drawn as an inset.
Merging them via min/max would silently absorb the case where a child escapes
its parent's window — the single most decision-relevant fact in hierarchical
planning, and the thing no conventional Gantt surfaces.

Breach is detected in both directions independently (a child starting before the
planned start, or ending after the planned end).

## Scope

- `dataentryconfig.Gantt` + validation, mirroring `Kanban` / `Calendar`
- Recursive tree build honouring `multi_parent` and `on_cycle`
- Recursive roll-up fold; planned/rolled kept separate
- SPA `GanttView.vue`: outline tree + time bars, drill-down with breadcrumbs,
temporal zoom, expand/collapse in place, peek-ahead segments
- Breach rendering with colourblind-safe textures + inline magnitude labels
- Navigation entry + `permission:` gating, like other views

## Out of scope (see the feature)

Dependency arrows, drag-to-reschedule, critical-path scheduling.

## Open questions for planning

- **ACL orphan policy.** A hidden node mid-chain disconnects the visible
subtree below it. Promote orphaned descendants to the nearest visible ancestor,
or drop the subtree? Dropping is consistent with the row-level rule ("a hidden
entity is nonexistent"); promotion preserves more information. Both are
defensible and they leak differently — this needs a stated decision, not
emergent behaviour. Note roll-up is therefore **per-principal** and cannot be
cached globally.
- **`roll_up: derive | strict` per source?** The breach semantics assume a
parent's own dates are a commitment. That is right for a project with a delivery
date, wrong for a container that is merely a grouping — which would nag about a
breach that is not one.
- **Should `multi_parent: duplicate` exist at all?** It works and is badged in
the prototype, but the same bar appearing twice under different rolled-up
ancestors is genuinely confusing, and roll-up double-counts. `first` may be the
only sane default, with single-parent enforcement the honest alternative.
- **Server-side vs. client-side roll-up.** Folding a deep subtree per render in
the SPA may not survive real project sizes.

## Prototype findings worth carrying forward

- **Peek-ahead earns its place.** Children rendered as a recessed strip inside
a collapsed parent's bar mean a collapsed node is not an opaque box. It was the
least certain feature and turned out the most useful.
- **Drill-down genuinely solves unbounded depth**, and must allow *skipping*
levels — clicking a deep row jumps straight there.
- **The two breach types must differ by texture, not hue.** Red/amber is the
classic deuteranopia collision. The prototype uses a dot field for "child
outside planned window" (internal inconsistency) and diagonal stripes for "past
committed date" (broken external promise), verified under a full greyscale
filter. They also need separate tiers — stacked in one rectangle they are
unreadable, and they do co-occur.
- **Breach labels belong on the timeline, not in the tree column**, anchored to
the region and carrying magnitude ("25d over"), not just a flag. Narrow regions
cannot contain their label, so labels anchor to the region's inner edge and run
toward the chart centre.
