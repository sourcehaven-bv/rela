---
id: DOCS-NDVC7W
type: docs-checklist
title: 'Docs: Document and e2e-test Create menu linking on entity pages'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The spec header states which relations the fixture page offers and why the third
tab makes no link. `SpacesPage.createFromMenu` documents that it returns after
the page link, because the SPA links before it navigates.

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

No README, CLAUDE.md or CLI change applies.

## External Documentation

- [x] Changelog entry added
- [x] API docs updated (if applicable)

The repo keeps no changelog file. GUIDE-data-entry (generated to
`docs/data-entry.md`) now covers Create menu linking in § Entity pages and the
`links` field in § The sidebar response.
