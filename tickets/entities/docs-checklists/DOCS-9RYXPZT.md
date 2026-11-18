---
id: DOCS-9RYXPZT
type: docs-checklist
title: 'Docs: TKT-JEAR2UA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the root uses a guard and fills the space store, why the remembered space is only a hint, why the pinned band is dropped when empty, why a top-level `builtins:` is refused with `spaces:`
- [x] Function/type docs if public API: godoc on `BuiltinRow`, `BuiltinConfig` (UX filter, config not secret), `Builtins.Hidden`, `SidebarResponse.HiddenBuiltins`, `Config.Builtins`, `Space.Builtins`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: reuses the nav `permission:` UX filter pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: new "Built-in sidebar rows (`builtins:`)" section, `builtins` row in the Spaces table, app root landing and a portal/back-office example in Spaces; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: UX filter only; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): `hidden_builtins` on `GET /api/v1/_sidebar` and `builtins` on `_config` documented in the guide
