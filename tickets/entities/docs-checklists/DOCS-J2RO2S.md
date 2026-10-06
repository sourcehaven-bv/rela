---
id: DOCS-J2RO2S
type: docs-checklist
title: 'Docs: Boot a SQLite project from its database: schema and acl.yaml through the layered config loader'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README describes the sqlite build without boot-order detail; nothing changed for readers)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flags or commands added by this ticket)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository has no changelog; release notes come from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API changes)

Code: godoc on `Config.projectConfig`, `readACLPolicy`, `loadMetamodel`,
`openDatabase` and the sqlite recipe explains why the database opens before
`prepare()`. Updated: CLAUDE.md storage section (7f2902d6: "sqlite can carry the
config in `rela.db`"), `docs/sqlite-backend.md` and GUIDE-sqlite-backend ("What
still lives on disk"). Decision: DEC-R9M57Z.
