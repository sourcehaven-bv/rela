---
id: DOCS-9T0A0J7
type: docs-checklist
title: 'Docs: Sankey view for data-entry'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious: the ACL order and the id-bounded pip read in `sankey_handler.go` and `sankey_pips.go`, the layout steps in `sankeyLayout.ts`
- [x] Function/type docs for the config types in `dataentryconfig/sankey.go` and the wire types in `apiwire/v1`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the gantt pattern already documented)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): `docs-project/entities/guides/GUIDE-data-entry.md` has a new "Sankeys" section with config, pips, access control and field tables
