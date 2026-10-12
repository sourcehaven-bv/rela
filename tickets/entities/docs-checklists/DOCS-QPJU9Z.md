---
id: DOCS-QPJU9Z
type: docs-checklist
title: 'Docs: Live views 3/3: MCP watch tool'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (watchHandler limits and per-process key, the subscribe-before-first-pass order, the deadline-bounded pass, retry of incomplete passes, cursor completeness in result, mcpWorldGate)
- [x] Function/type docs if public API (Deps.Changes, Deps.Rebind, ChangeSource, GatedReadBundle.Rebind)

## Project Documentation

- [x] ~~README updated~~ (N/A: no setup change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting rule; the ACL reasoning is in docs/acl-security.md)
- [x] Help text accurate (the watch tool description and serverInstructions match docs/mcp-server.md; tools_list golden regenerated)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (docs/mcp-server.md: "Watching for changes" with protocol, limits and proxy note; docs/acl-security.md: watch paragraph on gating, per-pass access, world grants, principal_property exception, timeout bound, search exclusion)
