---
id: DOCS-RTSUPP
type: docs-checklist
title: 'Docs: Database-backed comment stores: pgcomments and sqlitecomments over an injected pool'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

Both new packages carry a package doc explaining WHY they exist, not just what
they do — the postgres one names the multi-process defect, the sqlite one
explains why it went into `rela.db` despite being single-process. Four
non-obvious decisions are commented at the point a reader would otherwise "fix"
them:

- `sqlitecomments.timeFmt` — why it is deliberately NOT sqlitestore's
RFC3339Nano, with the exact lexical-vs-chronological failure.
- `facePrefixPattern` (both) — why the id is LIKE-escaped.
- `pgcomments.Rename`'s `$3::int` cast — pgx infers an untyped parameter's OID
from its use, and `substring(text FROM int)` is ambiguous enough that the driver
tries to encode the Go int as text and fails outright.
- `migrations/0015_comments.sql` — why there is no FK to entities (the service
owns comment lifecycle) and why the anchor is JSONB (adding an anchor kind must
not migrate stored comments).

## Project Documentation

- [x] README updated (if applicable) — N/A, no README-level change
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate~~ (N/A: no CLI surface changed)

CLAUDE.md gained a comments rule in the storage-backends section, beside the
`state.KV` rule it parallels: the backend-selection seam
(`backendOverrides.commentStore`, nil selects filecomments), the
`commentstest.RunAll` requirement for any new backend, the arch-lint ban on
importing `internal/store`, and the question that split the two database tiers —
"is this about the machine or about the content?".

The package table was deliberately left alone: `internal/comments` was never in
it, and the storage-backend section is where a reader looks for this.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo keeps no CHANGELOG; releases are
cut from git history)
- [x] API docs updated (if applicable) — N/A, the HTTP surface is unchanged

User-facing guides updated at source (`docs-project/entities/guides/`) and
regenerated into `docs/`:

- **`GUIDE-comments.md` → `docs/comments.md`** — "Where comments are stored" was
actively WRONG for the postgres build after this change. Now a per-backend table
with the multi-process motivation.
- **`GUIDE-postgres-backend.md`** — `comments` added to the tables created on
first start.
- **`GUIDE-sqlite-backend.md`** — the "no shared runtime state" bullet said
settings and caches stay under `.rela/`, which now reads as covering comments
too. Corrected, with why comments went the other way.
