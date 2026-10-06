---
id: DOCS-GQ2O9E
type: docs-checklist
title: 'Docs: rela db dump / rela db load: export and import a project''s config (and data) to and from rela.db'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README links to `docs/sqlite-backend.md`, which covers the commands)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository has no changelog; release notes come from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API changes)

Code: godoc on `ImportMarkdownData`, `ExportMarkdownData`, `LoadProjectConfig`,
`DumpProjectConfig` and `audit.OpFSImport`. CLI help for `rela db load`
(`--from`, `--data`, `--force`) and `rela db dump` (checked with `rela db load
--help` on 2026-10-04). Updated: CLAUDE.md "Markdown import into SQLite" (fifth
raw-store exception, d988168c); `docs/sqlite-backend.md` and
GUIDE-sqlite-backend (`rela db load` / `dump`, and converting a markdown project
with `--data`).
