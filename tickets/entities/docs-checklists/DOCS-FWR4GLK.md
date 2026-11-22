---
id: DOCS-FWR4GLK
type: docs-checklist
title: 'Docs: TKT-AAOWVET'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why one line per batch, why only ErrForbidden writes a denied line, why the line is written after commit
- [x] Function/type docs if public API: `records.Deps.Audit`, `NewService`, `audit.OpAppendRecords`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-records.md` (append audit, which refusals are logged, batching advice) and `GUIDE-audit-log.md` (op table rows for `append-records`, `update-record`, `delete-record`; record lines written after commit); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; the denied line follows the existing existence-oracle rule)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
