---
id: DOCS-XPEKH4
type: docs-checklist
title: 'Docs: TKT-W5ZSYJ'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: moved versus message-only writes, why the wake comes from the gated replay, why busy lists get no fallback frame in topic rounds, the residual one-bit timing signal, the one-lock take
- [x] Function/type docs if public API: godoc on `TakeLive`, `LiveChanges`, `CreatedChannel`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` (busy boards refresh live on session moves, the 100-card watch limit, staleness for moves from other processes) regenerated with `just docs`; `docs/data-entry/api-reference.md` live views paragraph (fallback rule per round type)
- [x] ~~Security guide~~ (N/A: no ACL change; selection stays on gated rows)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no endpoint or wire change; the live frame contract is described in api-reference.md)
