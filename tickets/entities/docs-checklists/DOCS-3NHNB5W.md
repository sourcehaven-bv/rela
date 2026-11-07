---
id: DOCS-3NHNB5W
type: docs-checklist
title: 'Docs: TKT-PV9R0GK'
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the locking order in the sqlflowrun package doc (why the subject lock is per subject id, why prune takes the definitions lock exclusive), the KV write order in kvflowrun, the epoch rule in flowrun
- [x] Function/type docs if public API: package docs for flowrun, kvflowrun, sqlflowrun, pgflowrun, sqliteflowrun, sqlexec, pgexec, sqliteexec; godoc on every Store method with its errors; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing shared-SQL-backend and state.KV patterns; the arch-lint entry records who may import the store)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] ~~User guides~~ (N/A: nothing runs flows yet and the Flows guide says so; TKT-9IT3LN8 documents runs)
- [x] ~~Security guide~~ (N/A: no read path or ACL change; the store authorizes nothing and has no caller)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
