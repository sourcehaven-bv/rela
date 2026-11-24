---
id: DOCS-CVAYIGZ
type: docs-checklist
title: 'Docs: Quick entry form with a global hotkey'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: no README covers data-entry.yaml keys; the guide does)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] API docs updated (if applicable)

GUIDE-data-entry gained "Quick entry (`quick_entry`)": the keys, the `q` key and
`/quick`, the desktop shortcut, the accelerator rules and the `key: q` conflict.
`docs/data-entry.md` is regenerated. The `/_config` response gains
`quick_entry`, which mirrors the config block. examples/todo-app shows it.
