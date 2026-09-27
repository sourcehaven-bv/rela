---
id: DOCS-GS0C5P
type: docs-checklist
title: 'Docs: sqlite SQL pushdown and derived indexes'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported functions/types have godoc (`sqlitestore.Reconcile`, `appbuild.ReconcileDerivedIndexes`, `storetest.RunGraphDifferential`)
- [x] Non-obvious decisions explained in comments
- [x] ~~Package docs updated if package purpose changed~~ (N/A: no package changed purpose)

`sqlitestore/derivedschema.go` explains why the index DDL is built with the
query builder's own helpers: SQLite uses an expression index only when the query
spells the expression identically. `graphquery.go` explains the naive fallback
and why rows are read to the end before yielding. `graphSource` explains the
unary `+` that keeps the type index from driving MatchingIDs.

## Project Documentation

- [x] CLAUDE.md updated with new patterns (the derived-index rule now covers both database backends; the sqlitestore paragraph names the SQL path and the differential harness)
- [x] docs/ updated for changed behaviour (GUIDE-sqlite-backend: queries answered in the database, a "Derived indexes" section with `rela db reconcile`; regenerated `docs/sqlite-backend.md`)
- [x] ~~Architecture docs updated~~ (N/A: no package boundary or dependency change; arch-lint clean)

## External Documentation

- [x] ~~README updated~~ (N/A: no new user-visible feature beyond the backend guide)
- [x] CLI reference updated (`rela db` help text now covers the sqlite build and query indexes)
- [x] ~~API docs updated~~ (N/A: no HTTP/MCP surface change)
