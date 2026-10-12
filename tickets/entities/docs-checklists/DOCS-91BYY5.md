---
id: DOCS-91BYY5
type: docs-checklist
title: 'Docs: Section hide_empty'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — why the omit runs after
`resolveSectionCreate`, why incomplete sections stay, why each combination is
refused.
- [x] Function/type docs if public API — `ViewSection.HideEmpty`,
`validateSectionHideEmpty`, `omitHiddenEmpty`.

## Project Documentation

- [x] README updated (if applicable) — not applicable.
- [x] CLAUDE.md updated (if new patterns) — no new pattern.
- [x] Help text accurate (if CLI changes) — no CLI change.

## External Documentation

- [x] Changelog entry added — the repo keeps no changelog file.
- [x] API docs updated — `docs/data-entry.md` (generated from
`GUIDE-data-entry`): `hide_empty` in the section key table and a "Hiding empty
sections" section with the refusals. No wire field changed.
