---
id: DOCS-7UBB3UE
type: docs-checklist
title: 'Docs: TKT-P4GFG1'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a foreign push reaches only a read-state pile by key, why actors resolve through the gate, why read items are evicted first
- [x] Function/type docs if public API: godoc on piles.Declared, PushRequest, Service.Push and RenameOwner; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] GUIDE-metamodel (`piles:` and the `notify` action), GUIDE-data-entry (toggles, the notifications menu, privacy of declared piles), GUIDE-lua-scripting and GUIDE-mcp-server (declared keys); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: declared piles follow the existing piles read gate)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): docs/data-entry/api-reference.md covers the key, locked, unread and _read fields
