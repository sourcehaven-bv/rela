---
id: FEAT-AS2VRW
type: feature
title: Hierarchical Gantt views for data-entry
summary: 'Configurable hierarchical timeline views declared under gantts: in data-entry.yaml. Supports recursive self-referential containment (project inside project) to unbounded depth, navigated by click-to-drill rather than indentation. Parent spans roll up from descendants; an entity''s own declared dates are kept separate so children escaping the planned window render as a visible breach.'
description: 'An interactive hierarchical timeline (Gantt) view type for the data-entry web app, configured declaratively under gantts: in data-entry.yaml the same way kanbans: and calendars: are. Renders recursive containment (a project containing sub-projects to arbitrary depth) as an outline tree against a time axis, with derived roll-up bars, click-to-drill navigation, and explicit planned-vs-actual breach rendering.'
priority: medium
status: proposed
---

## Summary

An in-app, interactive **hierarchical timeline view type** for the data-entry
web app — entities laid out against a time axis, nested by a containment
relation — configured declaratively under `gantts:` in `data-entry.yaml`,
exactly the way `kanbans:` and `calendars:` are configured today.

## Motivation

rela can already show *status* spatially (kanban columns) and *dates* spatially
(calendar grids). What it cannot show is **structure over time**: which work
contains which other work, and how those nested spans relate.

This is the natural third member of the view family. The graph already carries
both edges it needs — containment is a relation type, and date properties
already exist — so this is a rendering capability, not a data-model change.

## Relationship to the sibling views

| | `kanbans:` | `calendars:` | This feature (`gantts:`) |
|---|---|---|---|
| Primary axis | Status (enum → columns) | Date (grid of periods) | Time (continuous) + hierarchy |
| Structure shown | Flat, optional swimlanes | Flat | Recursive tree, unbounded depth |
| Navigation | Scroll | Period paging | Drill-down + temporal zoom |

## Self-referential containment is the defining constraint

The hierarchy is **not** a fixed set of levels. A `project` may contain
sub-projects, which contain sub-sub-projects, and so on with no declared bound
(`contains: from: [project] to: [project]` is already legal — `blocks` proves
same-type relations work today).

This rules out the designs that assume nameable levels — two-level swimlane
roadmaps, physically nested containment boxes — and drives three requirements:

- **Depth is navigated, not displayed.** Indentation stops being legible past
three or four levels. Click-to-drill re-roots the view on any bar and rescales
the axis to that subtree, so render cost is bounded by fan-out at one level
rather than by tree size.
- **Roll-up is a recursive post-order fold**, not a per-level rule.
- **Cycles, multi-parent nodes, and mixed hierarchies must be handled
explicitly**, because the graph permits all three.

## Planned vs. rolled-up spans are kept separate

An entity carries up to three independently-optional date roles, mapped per
source type: `start`, `end`, and `committed`.

Two spans are computed and deliberately **not merged**:

- `planned` — what the entity itself declares
- `rolled` — the envelope of its descendants

The bar spans their union so an overrun is visible; the planned window is drawn
as an inset within it. If the two were min/max'd together, a child escaping its
parent's window would be silently absorbed — which is precisely the condition
the view exists to surface.

## Scope

- A `gantts:` block in `data-entry.yaml`, mirroring `kanbans:` structurally
- Per-entity-type source mapping of the three date roles
- Recursive roll-up of parent spans from descendants
- Click-to-drill navigation with breadcrumbs; temporal zoom (quarter/month/week)
- Expand/collapse in place, coexisting with drill-down
- Peek-ahead: children previewed as segments inside a collapsed parent's bar
- Breach rendering: children outside the planned window, and spans past a
committed date, as texturally distinct (colourblind-safe) annotations
- Explicit `multi_parent:` and cycle policies
- Sidebar/navigation integration and `permission:` gating, like other views

## Explicitly not in this feature

- **Dependency arrows.** Dependency is a DAG orthogonal to containment; beyond
a handful of edges it renders as spaghetti. It deserves its own view.
- **Drag-to-reschedule.** Dragging a parent under roll-up semantics is a
cascade write across descendants — a separate design conversation.
- **Critical-path scheduling.** Deriving dates from durations plus dependencies
is a constraint solver, not a view.
