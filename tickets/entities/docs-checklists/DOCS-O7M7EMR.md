---
id: DOCS-O7M7EMR
type: docs-checklist
title: 'Docs: View section permission and hiding sections with only redacted fields'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious — why withheld sections stay in the
slice until after `resolveSectionCreate`, why a relation field counts as
content, and that only the `_views` path omits a section.
- [x] Function/type docs if public API — `ViewSection.Permission` (a UX filter,
not an access control), `allFieldsRedacted`, `omitHiddenSections`.

## Project Documentation

- [x] README updated (if applicable) — not applicable.
- [x] CLAUDE.md updated (if new patterns) — no new pattern; it reuses the
`permission:` UX filter of navigation entries and dashboard cards.
- [x] Help text accurate (if CLI changes) — no CLI change.

## External Documentation

- [x] Changelog entry added — the repo keeps no changelog file.
- [x] API docs updated — `docs/data-entry.md` (generated from
`GUIDE-data-entry`): `permission` in the section key table and a "Sections the
reader cannot use" section. It states that the key protects no data and that
`visible:` protects field values. No wire field changed.
