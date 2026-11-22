---
id: DOCS-L4CBEAU
type: docs-checklist
title: 'Docs: TKT-CY0NTFS'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the value comes from the principal and not a stamped query identity, why a raw login yields "", why an empty value is left out of `set`, why the placeholder is engine-only, why the Configure judge exempts the exact target
- [x] Function/type docs if public API: godoc on `TemplateVars.CurrentUserID`, `PlaceholderCurrentUserID` and the `Interpolate` variable list

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: extends the trigger placeholder set)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-flows.md`: placeholder table row, the note-author example under relate, the empty cases, and which placeholders inline scripts get; `GUIDE-metamodel.md`: pointer from "Matching the current user"; regenerated docs/flows.md and docs/metamodel.md

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
