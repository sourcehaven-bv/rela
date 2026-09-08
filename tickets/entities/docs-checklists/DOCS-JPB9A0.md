---
id: DOCS-JPB9A0
type: docs-checklist
title: 'Documentation: face addresses and world affordances (TKT-SLFURL)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new convention beyond what the godoc on `entityRef` and `utils/entityRef.ts` records)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; the PR description carries it)
- [x] API docs updated: docs-project guides for content-states (Step 6), data-entry (worlds section, `_world.via`, `_faces[].ref`, new "Addressing a face directly" section) and metamodel (`banner` row); `docs/*.md` regenerated with `just docs`.
