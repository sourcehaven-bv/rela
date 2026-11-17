---
id: DOCS-7AAS994
type: docs-checklist
title: 'Docs: Conditional grant edges'
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not cover acl.yaml options)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: grant conditions follow the existing exact-lowering rule; no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flags changed; `rela acl audit` gains an info finding documented in the security guide)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog)
- [x] API docs updated (if applicable): GUIDE-acl-overview ("A grant that expires") and GUIDE-acl-security ("A grant with a when: condition"); docs/acl-overview.md and docs/acl-security.md regenerated with `just docs`
