---
id: DOCS-S1EVV7
type: docs-checklist
title: 'Documentation: SQLite connection split and migration ladder'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious — the two findings this diff fixed
  carry their reasoning permanently: why freshness must be measured before
  `schemaSQL` (afterwards the two cases are indistinguishable), and why
  `readOnlyDSN` escapes (URI mode reads `#` as a fragment, so concatenation
  addresses a different file). Both cite the measured failure, not the rule.
- [x] Function/type docs if public API — `Conn`, `Connect`, `New`,
  `Conn.Close`, `Conn.DB`, `Status` and the `migrations` ladder all document
  ownership, ordering and the forward-only contract.

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-facing surface change; the sqlite build
  is not yet a shipped binary target)
- [x] ~~CLAUDE.md updated~~ (N/A: the storage-backend section's description of
  sqlitestore is still accurate. The config-in-the-database pattern belongs
  there once Phase C makes it usable, not while only the table exists.)
- [x] Help text accurate — `rela db migrate` and `rela db status` now report
  real versions instead of prose, and their output matches the postgres build's
  wording.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no changelog file in this repo; the
  behaviour change is recorded on the ticket and in the PR description)
- [x] ~~API docs updated~~ (N/A: no HTTP or MCP surface touched)

## Note on a behaviour change

`rela db status` on the sqlite build could previously only succeed. It can now
exit 1 when the database is behind, matching the postgres build. That is the
point of the command, but it is a change for any sqlite-build pipeline that
relied on unconditional success — called out in the PR description.
