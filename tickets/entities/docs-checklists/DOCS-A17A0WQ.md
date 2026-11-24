---
id: DOCS-A17A0WQ
type: docs-checklist
title: 'Docs: Unset properties from a flows.yaml trigger'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the removal folds into the write in flight, why the output rendered last wins, why required and state-machine properties are refused
- [x] Function/type docs if public API: godoc on `PropertyWrites`, `Apply`, `Result.Writes`, `Result.HasWrites`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-flows.md`: `set` output with `unset:`, precedence, load checks; regenerated `docs/flows.md` with `just docs`
- [x] ~~Security guide~~ (N/A: a system write with the same posture as `set`)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
