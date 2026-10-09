---
id: TKT-TY6TBU
type: ticket
title: Reorder rows of a collection over the incoming side of an orderable relation
kind: enhancement
priority: medium
effort: m
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

## Description

TKT-RCRUWZ lets a reader reorder the rows of a list, board or table section that
shows one entity's outgoing edges over a relation declared `orderable: outgoing`
or `both`. A relation declared `orderable: incoming` or `both` also stores an
order for its target (`_order_in`), but no surface shows or edits it. A tab
scoped `{relation: X, direction: incoming}` and a section with one
`follow_incoming:` from `entry` list the sources that link to the page entity,
so they can show and edit that order the same way.

## Acceptance criteria

1. A list or board tab scoped to an anchor over an incoming relation
orderable on the incoming side shows its rows in the anchor's `_order_in` order,
valueless edges last.
2. Drag and keyboard moves on that tab persist after reload.
3. A table section whose rows come from one non-recursive `follow_incoming:`
from `entry` shows and moves in that order, unless it declares `sort:`,
`group_by:` or nested display.
4. A sort, a grouping or swimlanes turn the order and the handles off, as on
the outgoing side.
5. A principal who may not read `_order_in` sees no relation order; one who
may not write it, or may not update every edge a move may rewrite, has no
handles and gets 403 on a move.
6. A move considers only edges the principal can read; a hidden sibling ref
is the uniform 404; a densify rewrites only those edges.
7. A source that links from more than one face is addressed by the face of
the place the list shows.
