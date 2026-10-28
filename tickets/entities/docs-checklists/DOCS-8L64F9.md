---
id: DOCS-8L64F9
type: docs-checklist
title: 'Docs: Translate end-user screens of the data-entry SPA to Dutch'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (literal key records explain why keys are not built at run time; the editor command label lookup in rela-components explains why English stays in the command files)
- [x] Function/type docs if public API (new `useMessages` keys and functions in rela-components; `versionOpLabel`, `defaultBucketLabel`, `conflictMessage`)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not describe the interface language)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing rules in frontend/CLAUDE.md "Interface text and languages")
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (if applicable): GUIDE-data-entry "Interface language" now says only the Configure space and the Settings page stay in English; docs/data-entry.md regenerated with `just docs`
