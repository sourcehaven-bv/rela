---
id: DOCS-C9O0CA
type: docs-checklist
title: 'Docs: TKT-53FN0L'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why preset values never enter the draft, why earlier guards are checked, why the compose key travels with the post event
- [x] Function/type docs if public API: godoc on the compose flag and load checks; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` documents `compose: true` (strict boolean, left-out required properties, refused lists and guards, `confirm` dropped, undecidable earlier guards hide the button) with the branching example pinned by `TestDocsExample_Branching`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] OpenAPI description of the session button `compose` field in `internal/openapi/channels.go`; MCP session tool descriptions updated
