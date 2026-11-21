---
id: DOCS-HXVS1EZ
type: docs-checklist
title: 'Docs: TKT-3VLKP3D'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the hamburger sits in a sticky bar pulled over the pane padding, why the picker waits for `hidden_builtins`, why the empty date input sits under a Now button, why `primary` is refused outside lists
- [x] Function/type docs if public API: godoc on `BuiltinAsOf`, `BuiltinRows`, `ListColumn.Primary`, `refuseListOnlyColumn`; TS docs on `BuiltinRow` and `ListColumn.primary`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: extends existing builtins and list-only column patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: `as_of` in the builtins table with an explanation, `primary` in Column Options, new "Columns on a phone" section; regenerated docs/data-entry.md
- [x] ~~Security guide~~ (N/A: UX filter only; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: `hidden_builtins` already documented; it may now contain `as_of`, covered in the guide)
