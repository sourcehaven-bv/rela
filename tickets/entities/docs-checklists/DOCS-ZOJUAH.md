---
id: DOCS-ZOJUAH
type: docs-checklist
title: 'Docs: File properties show the file name in table and list cells (TKT-3J4YW9)'
started: "2026-10-16"
completed: "2026-10-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (`FileCellWidget`, `DENSE_HINT_WIDGETS`, the `file` branch in `denseHintKind`, `fileRefNames`, `attachmentFileUrl`)
- [x] Function/type docs if public API (`fileRefNames`, `attachmentFileUrl`, the `file` hint kind, `ViewTableCell.entityId`)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A)
- [x] CLAUDE.md updated (if new patterns) (frontend/CLAUDE.md: the dense `file` rule now names FileCellWidget)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-metamodel.md "File attachments and `max`": how a file property shows in cells. Regenerated into docs/metamodel.md with scripts/generate-docs.sh)
