---
id: DOCS-3Y4YHRN
type: docs-checklist
title: 'Docs: Computed group keys for data-entry lists'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why grouping runs on the redacted served page, one batch per group, first match wins, fallback
- [x] Function/type docs if public API: godoc on `ListGroup.ID`, `ListGroup.Condition`, `ListGroupBy.Computed`, `ListGroupConditionID`, `Activity.Group`; comment-lint gate clean

## Project Documentation

- [x] examples/todo-app README describes the model, derived Today, unset and repeat
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: reuses the list condition pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: new "Computed groups" section; regenerated `docs/data-entry.md` with `just docs`
- [x] ~~Security guide~~ (N/A: grouping reuses the list condition's redaction)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] `_group` documented on the `Activity` wire type
