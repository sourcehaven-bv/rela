---
id: DOCS-40VA2Q
type: docs-checklist
title: 'Docs: PostgreSQL read-path follow-ups'
status: done
---

## Code Documentation

- [x] Godoc on new exported symbols (`store.PositionQueryer`, `store.GraphPosition`, `pgstore.SearchTitles`, `pgstore.WithSearchTitles`, `metamodel.RankingTitleProperty`, sqlitestore capabilities)
- [x] Comments explain why, including the invariants (two-pass gated search, per-collection bodies, the SQLite gate)

## Project Documentation

- [x] docs/postgres-backend.md: search is ranked by title; what a common-term search still costs (source: GUIDE-postgres-backend)
- [x] docs/sqlite-backend.md: list pages answered by the database; relation-dependent reads still evaluated in process (source: GUIDE-sqlite-backend)
- [x] ~~CLAUDE.md~~ (N/A: the collection-reads rule from TKT-1U8XYN already covers these paths)
- [x] ~~docs/cli-reference.md~~ (N/A: no command change)

## External Documentation

- [x] ~~API reference~~ (N/A: no wire change; `_position` and `_search` responses are unchanged)
