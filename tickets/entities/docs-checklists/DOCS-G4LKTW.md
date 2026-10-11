---
id: DOCS-G4LKTW
type: docs-checklist
title: 'Docs: MCP tools for comments'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: commentFailure (two-step 500 literals), MCPComments.address (type from the stored row), commentTool (nil deps after reload), stdioComments (why list only)
- [x] Function/type docs if public API: CommentReader, CommentWriter, MCPComments, comments.View/AddInput/Accepted/OpError

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not list MCP tools)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern; reuses consumer-side interfaces)
- [x] ~~Help text accurate~~ (N/A: no CLI flag changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] API docs updated: docs/mcp-server.md "Comment Tools", docs/comments.md "The MCP tools"
