---
id: BUGA-VYEG2I
type: bug-analysis-checklist
title: 'Analysis: Remote MCP: lua_eval/lua_run bypass read ACL; search_entities returns unreadable hits'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (the new cmd/rela-server remote MCP tests fail against the pre-fix wiring)
- [x] Minimal reproduction steps documented (BUG-RIJR6R body; remoteMCPClient fixture)
- [x] Environment/conditions noted (rela-server -mcp with an acl.yaml policy; stdio unaffected)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned (AM-remote-mcp-read-surface)
- [x] Related areas checked for similar issues (other remote Deps fields: Store/Tracer/Validator already gated; counts raw by design; Config is not secret)
