---
id: DOCS-I1J9MU2
type: docs-checklist
title: 'Docs: TKT-EV8U8T4'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why `expire` is CLI-only, why decimals are re-decoded with UseNumber
- [x] Function/type docs if public API: godoc on RecordService, RecordReader, RecordsCmd and records.UserText; comment-lint clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] Help text accurate (if CLI changes): kong help on `rela records`

## User-facing Documentation

- [x] GUIDE-records, GUIDE-lua-scripting, GUIDE-mcp-server and GUIDE-cli-reference updated; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: reads use the existing owner gate)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no REST change)
