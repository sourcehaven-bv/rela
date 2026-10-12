---
id: DOCS-3TE737
type: docs-checklist
title: 'Docs: Live views 1/3: change feed with SSE resume'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (changefeed package doc, token layout, sseStream positions)
- [x] Function/type docs if public API (Bus, Subscription, Batch, Source)

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-facing setup change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the ACL reasoning is in docs/acl-security.md)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (docs/data-entry/api-reference.md SSE section; docs/acl-security.md resume-token paragraph)
