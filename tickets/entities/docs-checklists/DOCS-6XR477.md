---
id: DOCS-6XR477
type: docs-checklist
title: 'Docs: MCP tools for running actions'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented: `MCPActions`, `ActionRunner`,
`script.ActionInfo`, `ActionRunInput`, `ActionError`
- [x] Non-obvious decisions carry a WHY: confirm after the gates, degraded
script detail, tools registered even with no actions, raw body is a
re-serialization
- [x] ~~`CLAUDE.md` updated~~ (N/A: no new rule; the action gate rules in
`internal/dataentry/CLAUDE.md` still hold)

## Project Documentation

- [x] `docs-project` guide `GUIDE-mcp-server`: new "Action Tools" section and
the remote-only note
- [x] `docs-project` guide `GUIDE-data-entry`: new "Running actions from an
MCP client" section
- [x] ~~`README.md`~~ (N/A: no change to the overview)

## External Documentation

- [x] ~~Tool README~~ (N/A: no external tool changed)
- [x] ~~Suppression documented~~ (N/A: no new lint suppression rule)
- [x] ~~Migration notes~~ (N/A: additive tools, HTTP behavior unchanged)

## Verification

- [x] `just docs` regenerated `docs/mcp-server.md` and `docs/data-entry.md`
- [x] Documented parameters and codes match the tool schema and tests
- [x] Documented behavior was tested, not assumed (see IMPL-BQ22Y2)
