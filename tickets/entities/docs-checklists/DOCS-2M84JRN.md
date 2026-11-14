---
id: DOCS-2M84JRN
type: docs-checklist
title: 'Docs: TKT-M58EQ1W'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: migration 0047 (trigger lookup through TG_TABLE_SCHEMA, why the backfill runs outside the migration), the shadowed alias in worldPrimeSQL, the heal queue's gen column, SearchVisible's stop at the limit
- [x] Function/type docs if public API: godoc on `SyncSearchTitles` and `WithSearchTitles`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-postgres-scale-run.md`: search row with the new numbers, search removed from the growing endpoints, a paragraph on the remaining growth; `docs/postgres-scale-run.md` kept in sync
- [x] ~~Security guide~~ (N/A: no ACL change; search still applies visible: in the store query)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the search API is unchanged)
