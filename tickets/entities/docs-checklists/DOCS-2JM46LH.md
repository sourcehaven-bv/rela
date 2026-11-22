---
id: DOCS-2JM46LH
type: docs-checklist
title: 'Docs: TKT-VMSWSWP'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: godoc on RecordAggregator, WithRecords and RecordQuery says why the owner gate runs before the fold
- [x] Function/type docs if public API: godoc on the new types; comment-lint clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flag change)

## User-facing Documentation

- [x] GUIDE-metamodel and GUIDE-records updated; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: reads use the existing owner gate)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no REST change)
