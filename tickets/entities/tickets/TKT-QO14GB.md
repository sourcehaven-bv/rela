---
id: TKT-QO14GB
type: ticket
title: 'Owned items: relation targets that live only on the parent page'
kind: enhancement
priority: medium
effort: l
started: "2026-10-05"
completed: "2026-10-06"
status: done
description: Design and build a relation whose targets (e.g. subtasks) appear only on the parent page and whose text search resolves to the parent.
---

## Description

A relation whose target records exist only as part of their source record. The
motivating case is a task with subtasks, as in the Atlas Projects design
(`RlRelatedList` in rela-components). A subtask:

- shows only on its parent's page, in a related list with add and select;
- has no detail page, and does not appear in lists, kanbans, the sidebar,
pickers, dashboards or feeds;
- is still found by search: a hit on a subtask's text leads to the parent.

This ticket starts as a design discussion. Decisions so far are below; the
remaining open questions follow them.

## Decisions (2026-10-05)

- **Owned entities, not a list property.** Owned items carry assignees, dates
and status, and their assignees must see them (for example in "my tasks").
- **Owned-ness belongs to the relation, not the type.** A relation type is
declared `owning`; an entity with an inbound owning edge is owned. A subtask can
then be a `task` under a `task`, and promoting it means removing the edge, with
no type change. Moving it means relinking. Both are rare but allowed.
- **Visible, but as part of something larger.** Owned items are not hidden
from other surfaces. Wherever one appears (search, "my tasks", a list) it is
shown with its parent, and how that looks may differ per surface.
- **One navigation rule, per instance.** If the entity has an inbound
owning edge AND the principal can read that parent, every link to it opens
`parent#child`, and a direct URL to it redirects there. Otherwise it opens the
ordinary detail page. This covers three cases with one rule: a child whose
parent the principal cannot read (rare, but possible through assignment) gets a
working page instead of a dead link; a promoted child (edge removed) becomes a
normal entity; and nothing needs a second page type.
- **No exclusion mechanism.** A list shows its own type. With a recursive
model (`task` owning `task`) a task list includes subtasks, shown with their
parent. With a separate `subtask` type, task lists do not include them. Either
is ordinary schema/config.
- **Search returns the child as its own hit, carrying its parent.** The
result reads "child, in parent" and links to `parent#child`. The child's text is
NOT folded into the parent's search document: one shared index entry cannot be
filtered per principal, so folding would let a search reveal a child's text to
someone who can read only the parent. The child keeps its own normal read gate.
A query split across parent and child text does not match; that is accepted.
- **One level only.** An owned entity cannot own entities. Validation
rejects the second level.
- **Delete is transactional:** deleting a parent deletes its owned children.

## Existing building blocks

- `min_incoming: 1` / `max_incoming: 1` on a relation expresses "exactly one
parent".
- `inherit_roles_through` gives a child its parent's local roles (ACL).
- View sections with `create:` (`flow: modal`) already create a related
record inline without navigating.
- A `default` query scope hides a type from lists and kanbans, but not from
search, pickers, graph or MCP.

## Missing

- No owned/embedded flag on entity or relation types.
- Delete never removes related entities, only edges; no `deleted` trigger.
- Search hits always navigate to `/entity/<type>/<id>`.

## Merged

PR #1780 merged into `develop` on 2026-10-07 as 41886e10.
