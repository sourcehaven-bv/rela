---
id: DOCS-Z2EFSK
type: docs-checklist
title: 'Documentation: SQLite content versioning'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The three places a reader would otherwise reach a wrong conclusion each carry
a note saying why, because an ABSENT mechanism and a FORGOTTEN one look
identical in a diff:

- `sweep.go` — why there is no advisory lock (pgstore needs
  `pg_try_advisory_lock` because several processes may share a database;
  `sqlitedb.Open` refuses a second process outright, so single-sweeper is
  guaranteed by construction).
- `versionschema.go` — why `AUTOINCREMENT` is load-bearing rather than
  decoration (a plain `INTEGER PRIMARY KEY` reuses the rowid of a deleted
  newest row, which lets a new row sort before an old one and silently
  mis-fences the lineage), and the three deliberate divergences from pgstore.
- `migrate.go` — why the `ALTER TABLE` sits on the open path rather than in
  the ladder rung that conceptually owns it.

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

- `CLAUDE.md`: the storage-backends section said sqlitestore "has no
  versioning yet, so it currently has neither git history nor version
  history" — now false. The Content versioning / Relation versioning / Version
  purge bullets said "postgres only"; they now name both database backends and
  point at `storetest.RunVersionTests` as the shared contract.
- `docs/sqlite-backend.md`: the "what you give up" list claimed the SQLite
  build has neither git history nor content versioning and told readers to
  "use one of the other two" if an audit trail matters. Rewritten: history now
  comes from the database, and what is actually given up is the *review*
  workflow (queryable history, but no pull request to read). Shared runtime
  state stays on the give-up list, with the reason.
- `docs/cli-reference.md`: `rela history`, `rela relation-history`,
  `rela restore`, `rela relation-restore`, `rela history-purge` and
  `rela relation-history-purge` were each marked "PostgreSQL build only".
- CLI help text: the five "not supported on this backend" messages named a
  "PostgreSQL-build feature", which would have actively misinformed a SQLite
  user whose backend does support it. `rela db`'s messages are untouched —
  those really are postgres-only.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: this repo has no CHANGELOG; releases are
      generated from commit history — see `docs/releasing.md`)
- [x] ~~API docs updated~~ (N/A: no API surface change. The `/_history` and
      `/_relation_history` endpoints, their responses and the
      `history_enabled` config flag are all unchanged — this ticket only makes
      an existing endpoint answer on a backend that previously refused it with
      a Not Implemented status. Note `docs/data-entry/api-reference.md` does
      not document the history routes at all, so there is nothing there to
      correct; the user-facing
      statement about which backends serve history lives in
      `docs/sqlite-backend.md` and `docs/cli-reference.md`, both updated
      above.)
