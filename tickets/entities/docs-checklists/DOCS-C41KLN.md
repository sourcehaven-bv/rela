---
id: DOCS-C41KLN
type: docs-checklist
title: 'Docs: TKT-S9TLJJ'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why earlier entries hide a button, what counts as decidable at load, the 2^53 limit, why ref and markdown presets are refused
- [x] Function/type docs if public API: godoc on `NormalizeProps`, `GuardHolds`, `Offers` and the button types; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: the CLI only gains fields in JSON output)

## User-facing Documentation

- [x] `GUIDE-realtime.md` button section (shape, load checks, earlier-entry rule, who sees which button, behaviour change for branching types without buttons, planned work in TKT-53FN0L and TKT-TXZZZ2); example pinned by `TestDocsExample_Branching`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; buttons post through the normal gated path)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI schema gains `ChannelSessionButton`, `entry` and `_actions.entries`
