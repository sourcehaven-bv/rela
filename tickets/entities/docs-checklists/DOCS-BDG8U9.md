---
id: DOCS-BDG8U9
type: docs-checklist
title: 'Docs: Remote MCP over HTTP'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new exported surface — `dataentry.MCPPath`, `MCPHandlerFactory`, `App.SetRemoteMCP`, `mcp.Server.HTTPHandler`, `principal.Stamped`
- [x] Non-obvious decisions explained where they live — why stateless is required (not a tuning choice), why the factory returns `http.Handler` rather than an SDK type, why a ctx principal beats the construction-time one
- [x] Comments explain WHY, not WHAT

## Project Documentation

- [x] `docs/mcp-server.md` — new "Remote MCP (over HTTP)" section: the flag, why the JWT flags are mandatory, what a remote caller can do, differences from stdio
- [x] `docs/server-security.md` — the three known gaps, plus "No authentication" qualified as a statement about the DEFAULT (the JWT gate predates this change and the old wording had gone stale)
- [x] Audit semantics corrected — the old text said `principal.user` is always the OS user that launched `rela mcp`; true for stdio, wrong for HTTP, where it is the JWT-verified subject

## External Documentation

- [x] ~~Changelog entry~~ (N/A: release tooling generates it from commits)
- [x] Operator-facing behaviour documented — opt-in, refuses without JWT, ACL-gated per caller
- [x] Migration notes — none needed: the endpoint is absent unless enabled, so upgrade is a no-op
