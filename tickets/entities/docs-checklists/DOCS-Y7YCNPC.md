---
id: DOCS-Y7YCNPC
type: docs-checklist
title: 'Docs: Push desktop notification conditions into the store'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: MaxMatchesPerRule (why a truncated rule reports nothing), Config.lower (exact lowering versus superset prefilters, empty identity), evaluation.scan, the tracker's untracked marker
- [x] Function/type docs if public API: MaxMatchesPerRule, Result.Truncated, Result.EvalErrors, Config.Evaluate

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README content about desktop notifications changed)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing LowerScope pushdown pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable): docs/desktop.md (via GUIDE-desktop) documents the 1,000-match limit per rule and which conditions the database answers
