---
id: DOCS-FQXNIK
type: docs-checklist
title: 'Docs: writes require read access to the existing row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no user-facing change outside ACL docs)
- [x] CLAUDE.md updated (if new patterns) — internal/entitymanager/CLAUDE.md, PatchEntity ordering and the read gate rule
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no changelog file in the repo)
- [x] API docs updated (if applicable) — docs/acl-security.md, section "Writes need read access to the row"
