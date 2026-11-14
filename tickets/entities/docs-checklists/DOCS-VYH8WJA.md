---
id: DOCS-VYH8WJA
type: docs-checklist
title: 'Docs: TKT-CKLZMWB'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why firstNByTypes reads growing windows, why a window is refused with a Scope, why the derived query index ends in id
- [x] Function/type docs if public API: godoc on `nextaction.Want` and `CandidateFunc`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-postgres-scale-run.md`: next-action row and the list of endpoints that grow with the data; regenerated `docs/postgres-scale-run.md` with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
