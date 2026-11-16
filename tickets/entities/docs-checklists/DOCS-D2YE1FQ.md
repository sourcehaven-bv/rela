---
id: DOCS-D2YE1FQ
type: docs-checklist
title: 'Docs: Document and dashboard tabs on entity pages'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not describe page tabs or dashboards)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing aggregate and uniform-404 rules; no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the project keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable): GUIDE-data-entry documents the `anchor` parameter of
`/api/v1/_dashboard/card`, its 400 error types and the uniform 404, plus entity
pages with document and dashboard tabs and a new "Entity dashboards" section.
`just docs` regenerated `docs/data-entry.md`.
