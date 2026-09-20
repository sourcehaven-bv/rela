---
id: TKT-U9DYW4
type: ticket
title: 'PostgreSQL read-path follow-ups: keyset position, title-ranked free text, header-backed view collections, bounded gantt drill-down'
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

TKT-1U8XYN left read paths that still did work proportional to the type rather
than to the answer. Priority from the user: PostgreSQL first, SQLite second;
memstore may stay inefficient; fsstore only where easy.

## What shipped

1. **Position in the store.** `store.PositionQueryer` with a generic fallback; pgstore answers with one window-function statement over the same ordered select the list page uses. `_position` takes it only through the ONE eligibility gate it now shares with the list page (`listNarrowing` in `listpushdown.go`: view condition, query scope, world, then `planListPushdown`). Search scopes keep the Go path.
2. **Search ranking on the title.** `pgstore.SearchTitles` (type to title property, built from the metamodel in the postgres recipe, bound as parameters) ranks by `similarity(lower(title), needle)`; both builders share one expression. Gated search rows no longer carry bodies: the hidden-field verdict is decided from id and properties, and bodies are fetched only for the undecided hits, for exactly the (id, face) pairs the gated query resolved.
3. **Views read headers.** Traversal loads content-free rows; after the ACL gate, one batched read loads bodies for collections rendered by anything other than `table`, `properties`, `list` or `nested`. The command runner asks for whole entities.
4. **Gantt drill-down** reads headers and only the hierarchy edges touching its subtree.
5. **SQLite.** Native `ListEntityHeaders`; `GraphQuery`, `GraphQueryHeaders` and `CountMatched` run in SQL for a narrowly gated simple shape (type, scalar string equality, world, face allowlist, order, page), with a probe that declines when a sort key holds a float, list or object. Id batches travel as one JSON parameter, so a batch of any size is one statement.

## Measured (seeded perf project, scale 1, manager principal, idle machine, warmed)

| request | before | after |
|---|---|---|
| `_position` (list scope sorted by due) | 80 ms, whole type read and sorted in Go | 19 ms, one statement |
| `_search q=telemetry` (term in most rows) | 1,240 ms | 216 ms |
| `_search q=TSK-0500` (rare term) | – | 1.7 ms |
| `_views/project/PRJ-0001` | 149 ms wall | 19 ms wall |
| `_gantts/delivery?root=PRJ-0001` | 57 ms | 25 ms |

## Not done

- Full gantt forest: the roll-up needs every descendant.
- fs seeding: the profile shows bleve merging segments after one index update per entity; batching observers inside `Tx` is not an easy change.
- SQL evaluation of relation predicates on SQLite, which is what ACL-scoped reads need there. Own ticket.
- A common-term search still pays about 170 ms for the substring match over every candidate row.
- Noted while reviewing: pgstore applies property predicates to world CANDIDATES, graphquerynaive to the resolved prime. Not changed here.
