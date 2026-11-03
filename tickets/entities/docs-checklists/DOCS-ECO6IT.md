---
id: DOCS-ECO6IT
type: docs-checklist
title: 'Docs: TKT-852C0M'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why literals allow only string, markdown and enum kinds, why `today()` is checked first, why the compiled list is safe to read
- [x] Function/type docs if public API: godoc on `literalPost` and `propError`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` Triggers section states that literals follow the message type's rules, names the order of load checks and that the first service-rule refusal is reported; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API documentation~~ (N/A: no API change)
