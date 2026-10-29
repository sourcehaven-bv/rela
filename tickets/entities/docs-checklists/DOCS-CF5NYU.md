---
id: DOCS-CF5NYU
type: docs-checklist
title: 'Docs: Hide agent-internal message types from people'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: fold keying and the separate scroll anchor, kept rows for the visit, the silenced set for late announcements, why `display` restricts nothing
- [x] Function/type docs if public API: godoc on `MessageDisplay` and the `Display` setting; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` "Quiet message types" (values, folding, what stays shown, the toggle, not access control, which types qualify, limits tracked in TKT-O6RC8Z) and a pointer in `GUIDE-realtime.md`; example pinned by the docs example test; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: display only; no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: `_config` passes the new key through; no endpoint change)
