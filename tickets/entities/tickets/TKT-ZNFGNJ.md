---
id: TKT-ZNFGNJ
type: ticket
title: 'Relation-backed status: design boards and views for a single-valued status relation'
kind: enhancement
priority: medium
effort: l
started: "2026-10-06"
status: planning
---

## Description

Work out the design for **relation-backed status**: a task's workflow state is a
single-valued relation to a `status` entity instead of an enum property. This
ticket is the design. It covers how boards and the other views behave, and it
splits the result into implementable tickets. It does not ship code itself.

### Motivation

Atlas (Sourcehaven's ops graph) wants a board per initiative with its own
sections (for example Backlog, Postpone, In progress, QA) chosen from a library
or template. Today a kanban's columns come from one enum per entity type, so
every initiative shares one global set. Atlas discussion: `TASK-D1YRL` and
`TASK-24O71` in atlas, atlas PR #78.

Two approaches were weighed:

1. **A larger global enum with columns switched on or off per parent.** This
is purely a UI concern and the smallest change. It gives no metadata per status.
2. **Status as an entity (this ticket).** Each status carries a name, a
colour, an order and a category, and can later carry a description or a WIP
limit. Renaming a status does not rewrite tasks. A global, ordered set that only
a managing role may create avoids the "everyone adds columns" mess seen in Jira.

### Proposed model (to be validated)

- An entity type `status` with properties `title`, `order` (integer, global
sort key), `category` (an enum such as open, active, waiting or done) and
`colour`.
- A relation `task --has_status--> status` with exactly one target.
- An orderable relation `parent --offers_status--> status` (parent being an
initiative, project and so on) that selects and orders the columns for that
parent. A template is then just a preset of these edges.
- Properties of the status are read through the relation (relation paths, RES-8CKUNJ). Nothing is copied onto the task, and compatibility with enum-based logic is not a goal.

### Interaction with existing features

Survey of develop at `2f7579199`. "Today" is the enum-property behaviour.

| Feature | Today | Needed for relation-backed status |
|---|---|---|
| Kanban columns | `column_property` must be an enum (`validate.go:2196`); order from `columns:` or the enum | `column_relation`: columns are the targets, optionally taken from the page anchor's orderable relation (`offers_status`), ordered by `_order_out` or a target property |
| Kanban drag-and-drop | `onMove` sends a properties-only PATCH (`KanbanView.vue:626-645`) | Re-target the relation: an atomic remove-old/add-new PATCH (the form path in `relationsPatch.ts` already does this), with the optimistic update on `relations` |
| Kanban swimlanes | enum only | Probably out of scope; decide |
| Kanban create | board-level "New" only, no per-column default | Per-column create that prefills the relation (needed for the "Add section" flow as well) |
| List `group_by` | property, single-valued, `groups` need an enum (`validate_groupby.go:84-118`) | Group by a relation target; section order, colour and label taken from target properties; the create prefill sets the relation |
| Sorting | `SortSpec{property}`; relation columns are never sortable (`tableColumns.ts:85`) | Sort by a related property (`has_status.order`) in lists, sections and dashboards |
| Filter controls | relation filters exist but match on **display title**, eq/ne (`api_v1.go:566-640`); candidates are the first 100 of the type | Match by id. Per-parent statuses with the same title ("Done") would otherwise collide. Scope the candidates |
| Form picker | `RelationPicker` is single-select at `max_outgoing: 1`; candidates are every entity of the type; `FormRelation` has no Go `default` key (the SPA reads one that is silently dropped) | Scoped candidates (only what the parent offers), a working default (the parent's first status) |
| Cardinality | `max_outgoing`/`min_outgoing` are analysis-only (`cardinality.go:216`) | Enforce single-valued at write time, or define what a board does with 0 or 2 targets |
| Styles and badges | `styles:` keyed by enum type; page `badge:` must be a property | Colour from the target entity; depends on a colour property type (TKT-28FRME, FEAT-8OTJVW) |
| Transitions | state machines only on enum custom types (`statemachine/`); `_transitions` only for those | Open: allowed moves as edges between statuses, checked on `replace`, or no transitions in the first iteration |
| Automations | `relation_created`/`relation_removed` triggers; a re-point fires remove plus create | A "relation re-targeted from X to Y" trigger, or document the pair |
| Validations, `condition:` | `related()` works in `when_condition`, list and sidebar conditions, next actions and MCP | Mostly covered by `related()`; `key_props` in next actions is property-only |
| Dashboard and search | breakdown counts `properties[group_by]`; query syntax has no relation predicates | Relation-aware breakdown and query predicate; compare TKT-AIEGHU |
| Gantt, calendar | tooltips refuse relations; `filter_controls` are validated but not rendered | Low priority; note only |
| CalDAV | completion is a property (`caldav_mapping.go`) | Read `has_status.categorie` through a relation path |
| Computed properties | cannot read relations (`computed.go:119`) | Not needed: relation paths are read at request time |

### Missing capabilities, in order of importance

1. Kanban columns from a relation (`column_relation`), scoped to the anchor's
orderable relation.
2. Drag-and-drop that re-targets a relation.
3. Write-time enforcement of single-valued relations.
4. Scoped candidates for the relation picker and filter controls (anchor,
`query_scope` or `where`).
5. Relation filters by id instead of title.
6. List `group_by` on a relation.
7. Colour from a target entity (needs a colour type).
8. Sort by a related property.
9. Transitions and automation triggers for relation re-targeting.
10. Relation-aware breakdown, query syntax and `key_props`.

Related work: TKT-NC3D08 (relation card fields, done), TKT-5U7QBR and TKT-DL16XM
(relation filter controls, done), TKT-205V2N (`related()`, done), TKT-LPLZ1V
(kanban `condition:`, backlog), FEAT-FE5P (reorderable relations, proposed;
`orderable` is undocumented in docs/), TKT-ZAD9PS (per-parent rollup bar,
backlog).

### Open questions

- Is a global status set with per-parent selection enough, or do parents
need private statuses? (Global keeps sorting meaningful.)
- What does a parent's board show for a task whose status the parent does
not offer? Hiding it silently is not acceptable; an "Other" column or a
validation warning is.
- A task under two parents: which parent's column set applies? (Likely the
page's anchor; the status itself is global, so no conflict.)
- Settled in RES-8CKUNJ: no derived or copied property. Views read the status through relation paths.
- Is the generic name `column_relation` right, or should the kanban accept a
`columns_from: {relation, anchor_relation, order_by}` block?

### Scope

In scope: the design, a decision on how views read status properties, and a set of
follow-up tickets (one per missing capability) with acceptance criteria.

Out of scope: implementation, and multi-valued relations on boards (tags).

### Acceptance criteria

- A research or decision entity records the chosen model and the rejected
alternative (per-parent enabled enum values).
- Every row of the table above has a stated behaviour.
- Follow-up tickets exist for the capabilities chosen for the first
iteration, each linked to this ticket.
