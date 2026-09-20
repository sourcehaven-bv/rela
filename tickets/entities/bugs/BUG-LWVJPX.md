---
id: BUG-LWVJPX
type: bug
title: migrate_face reports only entity counts, so a caller cannot tell whether edges were carried
description: A migrate_face step reports "changed N record(s)" counting ENTITIES only. The same run rewrites every outgoing edge of those entities and says nothing about them. Before BUG-TOX8U4 that identical output accompanied 245 SILENTLY DESTROYED relations on a real dataset — the operator had to count rows in SQL to discover it. PR 1627 fixes the destruction but not the reporting, so the output still cannot distinguish "edges carried" from "edges lost".
priority: medium
effort: s
status: backlog
---

## Problem

`rela migrate data --apply` reports a `migrate_face` step as:

```
migrate_face   beleid.status → face      changed 18 record(s)
migrate_face   procedure.status → face   changed 43 record(s)
data schema in sync (shape d40c81bbdccd)
```

`changed 61 record(s)` counts **entities**. The same run also rewrites every
outgoing edge those entities own — 854 relations in the measured case — and
reports nothing about them.

## Why this matters, measured

On a clone of a real dataset (61 `beleid`+`procedure` rows at the zero
coordinate, 854 relations):

| binary | output | relations before → after |
|---|---|---|
| develop (pre-BUG-TOX8U4) | `changed 61 record(s)` / `data schema in sync` | 854 → **609** |
| PR 1627 | *byte-identical output* | 854 → 854 |

The destructive run and the correct run are **indistinguishable from the
output**. Exit code 0 both times, no warning either time. The 245 destroyed
edges were discovered only by counting rows in SQL, and the entity count was
`61` in both cases because entities were never the thing at risk.

## Scope

BUG-TOX8U4 (PR 1627) fixes the data loss: `migrateFaceStep` now re-creates each
deleted edge at the destination face. This ticket is the remaining
**observability** half — and it is what made the original defect invisible
rather than obvious. A migration that rewrites 854 rows should say so.

## Fix

`migrateFaceStep`'s `StepResult` should report edges carried alongside rows
moved, so the run output distinguishes the two. Something like:

```
migrate_face   beleid.status → face      changed 18 record(s), carried 232 edge(s)
```

`StepResult` already carries `Affected` and `Notes`; the step already iterates
`del.DeletedRelations` to re-create them (PR 1627), so the count is in hand at
the point it would be reported.

Worth considering alongside: a step that moves rows and carries edges could
assert conservation itself (edges out == edges in) and fail loudly rather than
leave it to an operator's SQL. That is a stronger version of the same idea and
may belong here or as a follow-up.

## Verification procedure this came from

Rehearsing a face migration needs these SQL checks precisely BECAUSE the output
does not answer them:

1. `SELECT count(*) FROM relations` before == after (catches a BUG-TOX8U4
regression).
2. Scoped tails: join `entities`, restrict to the types that gained faces,
assert every content-scoped edge sits at its entity's face and none at `''`.
(Edges belonging to a faceless type legitimately stay at `''` — scoping the
check to the migrated types avoids that false positive.)
3. No migrated-type rows left at the zero coordinate.

Steps 1 and 2 are what this ticket would let the tool answer for itself.

## Related

- **BUG-TOX8U4** / PR 1627 — the data-loss fix; preserves edges, adds no
reporting.
- **BUG-VFHUWO** — the routes that 404 on a faced address, which is why a
migration cannot be verified through the HTTP relation surface either.
