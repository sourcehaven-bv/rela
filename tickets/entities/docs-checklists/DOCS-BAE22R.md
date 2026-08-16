---
id: DOCS-BAE22R
type: docs-checklist
title: 'Docs: Remote MCP part 1 — ACL-gated read seam'
status: done
---

<!-- @managed: claude-workflow v1 -->

Backfilled. TKT-UIR41P shipped in PR #1338 as `status=done` without a docs
checklist, and the user-facing MCP guide was never updated for the ACL-gated
read seam. Unlike the sibling review checklists in this PR, this one is NOT
purely retrospective: the missing documentation was actually written here (see
TKT-W76LRP).

## Code Documentation

- [x] Godoc on `mcp.GraphReader` explains the narrow read capability and that a
visibility decorator satisfies the gated half
- [x] Godoc on `WithPrincipal` / `NewServer` records that a Principal is
required rather than silently degrading
- [x] `internal/mcp/request.go` documents the argument-access shim and why typed
In structs were deliberately deferred

## Project Documentation

- [x] ~~CLAUDE.md~~ (N/A: no new package or architectural rule; `internal/mcp`
was already in the package table)
- [x] ~~README.md~~ (N/A: generated; no project-level surface change)

## User-facing Documentation

- [x] `GUIDE-mcp-server.md` — new "Read gating (ACL)" section: the `GraphReader`
seam, row-level gating and `visible:` field redaction applying to MCP tools and
resources, the required principal, and the stdio principal identity
- [x] Scope stated honestly — stdio MCP is a local user-launched process, so the
gating is about consistency across read paths, not defending a network boundary
- [x] Cross-linked to `acl-security.md`; forward-references Remote MCP part 2
(TKT-BDG8U9) for the HTTP transport
- [x] `docs/mcp-server.md` regenerated via `just docs` from the source guide

## Pull Request
- [x] Docs written in the follow-up PR for TKT-W76LRP.
