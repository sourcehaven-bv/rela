---
id: DOCS-U81IT26
type: docs-checklist
title: 'Docs: list_entities filters answered in the store'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the fieldcond.go file comment explains the `(V and S) or (not V and S0)` rewrite; guardPage says why V reaches the store ungated; returnableFaces and absentLeaf state their exact cases
- [x] Function/type docs if public API: store.GraphQuery.RowCond, queryplan.LowerConditionExplained and UnsupportedConditionError, predicate.Program.ConditionTreeExplained, the new ConditionLowerer.Lower contract

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not describe MCP filters)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing visible: and collection-read rules)
- [x] Help text accurate (if CLI changes): the list_entities tool description lists the supported filter shapes

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): GUIDE-mcp-server (rendered to docs/mcp-server.md) lists supported and refused filter shapes and the per-caller refusal
