---
id: DOCS-UV3J23G
type: docs-checklist
title: 'Docs: validate reports deprecated data-entry syntax'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (configure.go: the call-site interface matches *migration.Error, which cmd/rela-server may not import)
- [x] Function/type docs if public API (CheckDataEntry names the migration error; migration.Error.Descriptions)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no flags or help text change; rela validate prints the existing migration message)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-data-entry.md: every `link: true` example is now `link: detail`, the column and section option tables document `link` as a string, and the best-practices item says `link: detail`; regenerated docs/data-entry.md)
