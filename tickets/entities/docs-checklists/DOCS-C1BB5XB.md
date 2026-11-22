---
id: DOCS-C1BB5XB
type: docs-checklist
title: 'Docs: Flows step form from a data-entry form'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not describe flows)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file; release notes come from PRs)
- [x] API docs updated (if applicable)

GUIDE-flows, GUIDE-data-entry and GUIDE-mcp-server describe `form:`, the
per-step reading of the form, `waiting.form_problem`, defaults and the Configure
checks; `docs/` is regenerated. The MCP tool descriptions name
`waiting.form_problem` and say which form rules the server does not enforce.
