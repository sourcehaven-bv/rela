---
id: DOCS-UKE8Z0
type: docs-checklist
title: 'Docs: Configurable backlinks section with per-type columns'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: the README lists guides, not view keys)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; the step follows the existing view and gate rules)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PRs)
- [x] API docs updated: docs/data-entry/api-reference.md "Backlinks in a view"

**Docs updated:** GUIDE-data-entry (mentioned_by step, per-type fields and
columns, mentioned_by_panel), GUIDE-acl-security (backlinks gating note),
docs/data-entry/api-reference.md, and an example in
prototypes/data-entry/project/data-entry.yaml. Generated docs regenerated with
`just docs`.
