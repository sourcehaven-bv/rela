---
id: DOCS-5YHTZR
type: docs-checklist
title: 'Docs: i18n infrastructure for the data-entry SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (resolution order and region borrowing in language.ts; why conflict/forbidden are unmapped in apiErrors.ts; the sync watcher in i18n/index.ts; the canonical-form rule in validateAppLanguage)
- [x] Function/type docs if public API (AppConfig.Language, SupportedUILanguages, localeMessages, NL_MESSAGES, chromeText)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not cover data-entry.yaml fields)
- [x] CLAUDE.md updated (if new patterns): frontend/CLAUDE.md "Interface text and languages"
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (if applicable): GUIDE-data-entry "Interface language" documents `app.language`, the resolution order and the Settings choice; docs/data-entry.md regenerated with `just docs`
