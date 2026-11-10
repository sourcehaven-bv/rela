---
id: DOCS-KJ222ZM
type: docs-checklist
title: 'Docs: TKT-P8H6XPW'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the hint is a disclosure and not a tooltip, why the label row never wraps, why the checkbox CSS reaches the row through `:deep`, why sanitization is tested under jsdom
- [x] Function/type docs if public API: godoc on `FormStep.Details`, `FormField.Hint`, `FormRelation.Help` and `FormRelation.Hint`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: "Help texts" section (table of the five places, link and Markdown guidance, fallback descriptions, why steps use `details`), field and relation key tables, step `details`; regenerated `docs/data-entry.md`
- [x] ~~Security guide~~ (N/A: no ACL or access change; help text uses the existing sanitized renderer)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated~~ (N/A: the config endpoint serves the new keys as part of the existing form config)
