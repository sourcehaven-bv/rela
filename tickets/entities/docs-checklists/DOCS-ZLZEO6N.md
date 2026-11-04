---
id: DOCS-ZLZEO6N
type: docs-checklist
title: 'Docs: next actions on every page'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: the README does not cover next actions)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the placement rules live in godoc and the guide)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] API docs updated: GUIDE-data-entry gains "Where suggestions appear" (`next_action_surfaces`, `suggestions`, `visible_on`, context sources, `cooldown` on them) and "The endpoint" (the GET parameters, `context_type`, the `seen` feedback kind); `docs/data-entry.md` regenerated with `just docs`
