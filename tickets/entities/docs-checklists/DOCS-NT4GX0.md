---
id: DOCS-NT4GX0
type: docs-checklist
title: 'Documentation: operator world chrome messages (TKT-5SZG2L)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: the rule "the words are the operator's or there are none" is recorded on the wire and metamodel types and in the guides)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; the PR description carries it)
- [x] API docs updated: metamodel reference (faces `messages.read_only`, worlds `messages`/`on_absent`, copies `on_success`, both load-check lists), content-states guide (Step 1 faces, Step 2 table, Step 5 table, Step 6 behaviour), data-entry guide (world-bound page, schema discovery); `docs/*.md` regenerated with `just docs`.
