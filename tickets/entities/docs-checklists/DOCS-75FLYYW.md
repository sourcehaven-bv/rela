---
id: DOCS-75FLYYW
type: docs-checklist
title: 'Docs: Fix _views path in retired-id redirect docs'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] ~~Exported functions/types have godoc~~ (N/A: no code change)
- [x] ~~Non-obvious decisions explained in comments~~ (N/A: no code change)
- [x] ~~Package docs updated if package purpose changed~~ (N/A: no code change)

## Project Documentation

- [x] ~~CLAUDE.md updated with new patterns~~ (N/A: no new pattern)
- [x] docs/ updated for changed behaviour
- [x] ~~Architecture docs updated~~ (N/A: no boundary or wiring change)

The acl-security guide now names the views route `/api/v1/_views/<type>/<id>`
(`docs-project/entities/guides/GUIDE-acl-security.md`, regenerated into
`docs/acl-security.md`).

## External Documentation

- [x] ~~README updated~~ (N/A: not affected)
- [x] ~~CLI reference updated~~ (N/A: no command or flag)
- [x] ~~API docs updated~~ (N/A: no behaviour, route or response change)
