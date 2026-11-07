---
id: DOCS-GK1031B
type: docs-checklist
title: 'Docs: TKT-H0TFDWU'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the switcher picks its control from the visible count, why an unreadable current world keeps it, why the face toggle switches world only to a readable world that heads the face, why previews get no toggle
- [x] Function/type docs if public API: godoc on `WorldDef.Label`/`Hidden`, `AppConfig.WorldSwitcher`, `worldSwitcherWire`; doc comments on `RlSegmentedControl` link mode, `WorldSwitcher`, `FaceToggle`, `faceToggleOptions`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: link mode follows the existing RlViewTabs `as`/`attrs` pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-metamodel.md` (`label` and `hidden` rows; hidden-default load check), `GUIDE-content-states.md` (key table, face toggle, world switcher), `GUIDE-data-entry.md` (new `app.world_switcher` section, unknown keys ignored); regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: `hidden` and `world_switcher` are presentation only; the docs say `hidden` is not a read boundary)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the new wire fields are optional and described in the guides; MCP `list_worlds` reports them)
