---
id: DOCS-L3YBKW
type: docs-checklist
title: 'Docs: Message options rendered as reply buttons'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: list normalization order (trim, NFC, cap, hazards, duplicates), why `into` must be a plain string, when choice buttons show (decisionCause at the head generation, none while `hasNewer`), joiner and variation-selector rules
- [x] Function/type docs if public API: godoc on `Choices`, `texthazard.IsDisplayHazard`, `MaxListItems`, `openapi.Limits`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel post` repeated `-P` for a list property; `--schemas` prints list limits and a `choices:` line; `GUIDE-cli-reference.md`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: list properties, `choices:` keys and load checks, reply buttons and Other, staleness and second answers, string to list config change; example pinned by `TestDocsExample_Choices`; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; buttons follow `_actions.post`)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI list property schema (`items`, `maxItems`, `uniqueItems`) and `choices`; MCP `post_message` and `read_channel` text; `GUIDE-mcp-server.md`
