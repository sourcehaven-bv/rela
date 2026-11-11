---
id: DOCS-RUYPH79
type: docs-checklist
title: 'Docs: Flows docs and CLI promise retries that do not exist (#2170)'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not mention flow retries)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from the PR)
- [x] API docs updated (if applicable)

GUIDE-flows ("Status", "Running flows", "Doing a step") and GUIDE-mcp-server
(`complete_flow_step`) are updated and regenerated into `docs/flows.md` and
`docs/mcp-server.md`. The `rela flow start` error, the `complete_flow_step` MCP
description, the `flowview` and `api/flows.ts` field docs, and the web app
strings say the same. TKT-FWQZGAM carries a note to restore the retry text when
the worker lands.
