---
id: DOCS-PVJYLB
type: docs-checklist
title: 'Docs: SQLite backend wiring (TKT-L1A3PH)'
status: done
---

<!-- @managed: claude-workflow v1 -->

This is where the SQLite backend becomes user-visible, so the docs deferred from
TKT-G91TBK land here.

## Code Documentation

- [x] `appbuild_sqlite.go` states why the DB lives under `.rela/` and why bleve
rather than FTS5 (search.Visible wraps any Searcher, so a native searcher is an
optimization not a prerequisite).
- [x] **Reasoning is documented where it would otherwise be undone.**
`releaseLoadedProject` explains the close-before-open ordering at the site
someone would reverse it — the old order looked harmless and was, until a
backend held an exclusive resource.
- [x] The two widened tags say why they widened, including what the old message
would have claimed (`db_nonpostgres.go` would have told sqlite users they were
on the filesystem backend).
- [x] `bleveindex_shared.go`'s tag is the explicit union it claims to be, with
a note that a tag not stating its own requirement is one someone narrows without
noticing.
- [x] `checkSchemaVersion` records why the marker matters more than the ladder:
`CREATE TABLE IF NOT EXISTS` is a silent no-op against a different shape, so
without a version an old database opens happily and fails at first query.

## Project Documentation

- [x] `docs/sqlite-backend.md` — generated from `GUIDE-sqlite-backend`,
paralleling `postgres-backend.md`. Covers what still lives on disk, the
single-writer refusal and *why* it is a refusal rather than a warning, the
network-filesystem limitation, and what you give up versus each of the other two
backends.
- [x] **It states plainly that this backend has neither git history nor version
history today.** The filesystem build gets history from git; postgres replaces
it with content versioning; SQLite has neither yet. Anyone who needs an audit
trail is told to use one of the other two rather than discovering it after
committing data.
- [x] `CLAUDE.md` — storage-backend table row, the single-process rationale,
and the extended isolation rule (no build but sqlite may link the driver).
- [x] `README.md` — index regenerated; CI enforces `docs/` matches
`docs-project/`.

## Knowledge Capture

- [x] The three inherited `!postgres` no-ops each carry their justification,
now with the correct scope: one writer *per project database*, not one process
per machine. That precision is load-bearing, because `derivedschema_nosweep.go`
warns a future reader that removing the lock creates a silent correctness hole —
a warning only actionable if its scope is exact.
- [x] The negative CI assertions carry a comment explaining that the
redeclaration error IS the mutual-exclusion mechanism, so a future "fix" that
narrows a tag does not silently degrade it.
- [x] `db_sqlite.go` records what would trigger a real migration ladder (the
first `schemaVersion` bump) rather than leaving it as folklore.
