---
id: DOCS-6TQARTV
type: docs-checklist
title: 'Docs: Flow tasks as a next-action source'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows existing next-action and flow patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] API docs updated (if applicable)

docs/data-entry.md: new "Waiting flow steps (`source: flow_tasks`)" section with
a banner example for a customer portal, the `source`/`flows` fields, and
`refresh_types` in the API part. docs/flows.md: pointer from "Doing a step" and
the `?run=` link.
