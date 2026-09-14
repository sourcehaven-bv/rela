---
id: TKT-MJKZQ3
type: ticket
title: 'display: nested — a two-level parent-child view section'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Add a `display: nested` value for a view section, rendering a two-level
parent→child tree. A `project` view configured with it shows its epics as
expandable rows, each expanding to its tasks.

A per-parent rollup bar is **TKT-ZAD9PS**, deliberately split out: it carries
all of this feature's ACL-fold and enum-validation risk, and the nested view is
useful without it.

This is a new section display in the existing `views:` system — the same place
`display: properties`, `cards`, `list` and `table` already live. No new layout
mechanism, no metamodel change, no merge machinery.

## Why a view section, not an extension to the generated page

`views:` sections are already rela's layout system for a detail page: `display`
picks the section type, `fields[].span` positions fields side by side in a grid,
`render`/`widget` control presentation. A nested section is another `display`
value in a system built for exactly this.

Defining a view for a type replaces the generated page
(`internal/dataentry/views_handler.go:505-509` — a plain either/or with one
caller of `buildDefaultViewConfig` and no merge). That is the normal trade for
any type laid out deliberately, not a defect to work around. The generated page
is the fallback for types nobody has styled; a project overview is by definition
a styled page.

Both paths converge on the same `ViewConfig` type and executor, so nothing here
is closed off to the generated page later.

## Config shape

```yaml
views:
  project_overview:
    entry: { type: project }
    traverse:
      - { from: entry,  follow: has-epic,  collect_as: epics }
      - { from: epics,  follow: has-task,  collect_as: tasks }
    sections:
      - heading: Epics
        source: epics
        display: nested      # NEW
        children: tasks      # NEW — which collected bucket is the child level
        columns: [status, due, owner]
```

One new section key (`children:`). `traverse` already names the containment
relations explicitly, so no separate `hierarchy:` declaration is needed.

## Child sort order — deferred

Children render in traversal/store order. **Sorting is deliberately not in this
ticket.**

`ViewSection` has no `Sort` field today
(`internal/dataentryconfig/config.go:1301-1311` — only `Heading, Source,
Display, Render, Fields, Columns, GroupBy, EmptyMessage, Link`), and nothing in
the view path sorts at all: there is no `sort.` call in
`internal/dataentry/sections.go` or `views.go`. The `sort: []SortSpec` fields
that do exist belong to `List` (`config.go:587`) and dashboard cards
(`config.go:1229`). An earlier revision of this ticket claimed section sort
"already works"; that was wrong.

Adding the field and a sort step would be straightforward, but it could not yet
deliver the order actually wanted. Enum comparison is byte-wise on the string
form (`internal/dataentry/api_v1.go:2148-2172`), so a `status` sort yields
`blocked, doing, done, todo` — alphabetical, not workflow order. "Completed
last" needs **TKT-9OFGH4** (sort enum properties by declared order), which must
land on both sides of the pushdown decision at once: the in-Go sorter and the
SQL `ORDER BY` in `listpushdown.go` / `store.GraphQuery.OrderBy`. Changing only
one makes a list read differently depending on whether its request happened to
be pushdown-eligible — exactly what the byte-wise rule was chosen to prevent.

So section `sort:` is folded into TKT-9OFGH4 rather than built here against a
comparison rule that is about to change. This ticket ships with unsorted
children and says so.

## Scope

In scope:

- `display: nested` with a `children:` section key naming the child bucket.
- Child rows rendered in traversal order (unsorted).
- A nesting shape on the section wire type, and `buildSections` emitting it.
- Node budget and truncation signalling.

Out of scope:

- More than two levels. Renders exactly parent → child.
- Auto-emission on the generated detail page (`buildDefaultViewConfig`). Possible later; both paths share the executor.
- A standalone route or sidebar entry. `views:` have no navigation entry kind and that stays true.
- Any per-parent rollup or aggregate (TKT-ZAD9PS).
- Child sorting, including a `Sort` field on `ViewSection` (TKT-9OFGH4).
- Any write affordance. Sections stay read-only.

## Bounding: node budget, not paging

The `_views` endpoint has **no paging and no node budget** today — only a depth
cap of 10 (`internal/dataentry/views.go:129-140`). Every child of every parent
loads in full, so a 1,412-task project would ship all of them.

This follows the gantt instead of inventing paging:

- A shared node budget (gantt default 2000, `internal/dataentryconfig/validate_gantts.go:29`).
- Per-parent `has_more_children`, because children are `omitempty` and a capped parent is otherwise byte-identical to a childless one.
- A response-level `truncated` flag, set only when a **visible** node was denied emission — never merely because the budget hit zero, so it cannot leak how close a principal's tree is to the cap.

There is deliberately no inline "load 25 more": over-budget parents show a count
and link to the child list. A per-parent paging parameter is a possible
follow-up, designed against real usage rather than speculatively.

## Design constraints

- **Children resolve against the already-gated collection, never the store.** `PolicyReader.Filter` calls `r.redacted()` on each surviving row (`internal/visibility/policyreader.go:88-93`), so every member of `viewResult.Collections` is row-gated AND field-redacted before a section builder runs (`internal/dataentry/views.go:96-99`). The nested arm must resolve child ids against that filtered collection; ids present in the retained attribution map but absent from it are exactly the hidden children, and dropping them IS the gate. No new ACL code — but the property must be pinned by a test, because it is what makes TKT-ZAD9PS's rollup safe to add later (the `CLAUDE.md` gate-before-fold rule).
- **Collection reads are content-free and batched.** Child rows need only id, title and the configured columns — `store.EntityHeader`, never a body. Cost must stay constant in row count.

## Acceptance criteria

1. A section with `display: nested` renders parent rows, each expanding to its own children, correctly attributed (parent A's children never appear under parent B).
2. Config load rejects `display: nested` when `children:` names a bucket absent from the view's `traverse`, and rejects `recursive: true` on a traverse feeding a nested section.
3. Children render in a deterministic order (traversal order) so the response is stable across identical requests.
4. A child the principal may not read is absent from the child list.
5. A parent whose children were withheld by the budget is distinguishable from one with no children (`has_more_children`).
6. `truncated` is set only when a visible node was denied emission.
7. Store reads for the section are identical at 10 and 50 parent rows (`storetest.Counting` budget test, as `TestQueryBudget_ListPageIsSizeIndependent` does at a pinned 6).
8. Child rows carry no markdown body. Note `include_content` is a LIST-path parameter (`internal/dataentry/rowcontent.go`) and does not exist on the `_views` path; section content is opt-in by `display` instead — only the `content`/`cards` arms set it (`sections.go:373`), while `buildSectionEntityData` (:234) sets no `Content` at all. So the criterion is that the `nested` arm does not populate content, not that it honours a parameter.
9. Attribution is not duplicated when a traverse needs more than one fixpoint pass.

## Prior art in-tree

- `mockups/project-overview/` — static mockups. `6-detail-widget.html` is the target shape; `1b-accordion-at-scale.html` shows behaviour at 84 epics / 1,412 tasks. Note both mockups draw an inline "Load 25 more" that this design deliberately drops.
- `internal/dataentry/gantt_handler.go` — gate-before-fold order and budget accounting.
- `internal/dataentryconfig/config.go:951` — the gantt's `hierarchy:`, for why containment is declared rather than inferred.
- `internal/dataentry/views_handler.go:505` — where configured and generated views converge.

## Measured evidence against inference

Auto-detecting containment from the metamodel was tested against the real
47-relation `tickets/schema.yaml` and rejected:

- "target type has any outgoing relation" → **103 nested sections** (a bug page would get 20, doc-task 12, ticket 9).
- "relation has `max_incoming: 1`" → **0 matches** in both `tickets/schema.yaml` and `docs-project/schema.yaml`; the key is never used, so the rule would ship as a no-op.

Hence explicit declaration via `traverse`.
