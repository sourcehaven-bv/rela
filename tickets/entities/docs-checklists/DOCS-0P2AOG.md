---
id: DOCS-0P2AOG
type: docs-checklist
title: 'Docs: Comments over stdio MCP'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (watchRun.commentsDue and why it is apart from the pass kind; filecomments write lock; asserted_by log line)
- [x] Function/type docs if public API (comments.ValidAuthor, Refusal, AddInput.Anchor, BuildTextAnchor; filecomments.Locker, WithLocker; mcp.CommentAdder, CommentDeps)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule)
- [x] Help text accurate (if CLI changes) (no new flags; `rela mcp --run-as` already documented)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (docs/mcp-server.md comment tools and watch targets; docs/comments.md MCP tools and job-identity authors; docs/server-security.md integration authors)
