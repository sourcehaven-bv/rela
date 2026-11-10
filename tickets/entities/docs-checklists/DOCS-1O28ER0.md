---
id: DOCS-1O28ER0
type: docs-checklist
title: 'Documentation: Flows M2: step forms and run UI'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not list subsystem features)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern; the flow routes follow the existing data-entry and MCP rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI command changed)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (if applicable)

GUIDE-flows has a "Doing a step" section with the web app, the HTTP route table
and the `problem` field. GUIDE-mcp-server lists the three flow tools.
GUIDE-data-entry has a Flows section. `just docs` regenerated docs/flows.md,
docs/mcp-server.md and docs/data-entry.md.
