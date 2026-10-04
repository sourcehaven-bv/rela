---
id: DOCS-Z5A80D
type: docs-checklist
title: 'Docs: Data-entry, templates and scripts read config through the services'' config loader'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no change visible at README level)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes in this ticket)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository has no changelog; release notes come from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP API changes; `/_custom/` and apps routes are unchanged)

Code: godoc on `lua.ReadDeps.Files`, `lua.ProjectFiles`, `ReadDeps.ReadScript`,
`config.Stater`, `config.DirLister` and package `rootfs`. `.go-arch-lint.yml`
documents the `rootfs` component. Updated: CLAUDE.md storage section (7f2902d6)
lists every reader that goes through the loader and says not to swap `rootfs`
for `FSLoader`; `docs/sqlite-backend.md` and GUIDE-sqlite-backend list the
config the database can carry.
