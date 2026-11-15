---
id: DOCS-TQA86WC
type: docs-checklist
title: 'Documentation: Content-free keyset pages for search with no text'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (HeaderPager godoc says why it is a separate interface and why fsstore has no native method; bodywatch headerPageOf says why it checks the innermost store)
- [x] Function/type docs if public API (HeaderPager, EntityPager, ListEntityHeadersPage, RunHeaderPageTests)

## Project Documentation

- [x] ~~README updated~~ (N/A: internal store read, no user-facing surface)
- [x] CLAUDE.md updated (if new patterns) (.claude/rules/collection-reads.md names store.ListEntityHeadersPage as the content-free keyset page read)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no user-visible behaviour change; search results and paging are identical)
- [x] ~~API docs updated~~ (N/A: no API changes)
