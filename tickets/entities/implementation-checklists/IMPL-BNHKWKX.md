---
id: IMPL-BNHKWKX
type: implementation-checklist
title: 'Implementation: Remote MCP toggle and tool filter in .rela/config.yaml'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: YAML and flag literals)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolation)
- [x] ~~Property comparisons use original object~~ (N/A: tool-name and error assertions)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- Headless rela-server with JWT identity against a local JWKS and a signed token, POST tools/list to /api/v1/_mcp:
  - `remote_mcp.enabled: false` with `-mcp`: 404, WARN names the config source.
  - `enabled: true` with `allow: [schema, show_entity, list_entities]`: 200, exactly those three tools.
  - nothing set: 404.
  - `enabled: true` with `RELA_MCP=0`: not served, WARN names $RELA_MCP.
  - `RELA_MCP=true`: startup refused ("want 1 or 0").
  - `deny: [run-action]`: startup refused, error lists the tool names.
  - top-level `remote-mcp:`: startup refused ("spelled remote_mcp").
- Mutation checks: removing one catalogue entry fails TestKnownToolNames_MatchesRegistrations; the watch replay check fails without the reads cleanup.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one resolution function for all three sources; one filter hook in addTool)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
