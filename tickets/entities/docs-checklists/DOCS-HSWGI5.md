---
id: DOCS-HSWGI5
type: docs-checklist
title: 'Docs: Comment and relation counts on kanban cards'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Exported symbols documented: `comments.Store.Count`, `v1.RowCounts`,
`KanbanCardField.Display` and `.Comments` carry doc comments
- [x] Non-obvious decisions carry a WHY: `serveCommentCounts` explains the
global-only `comment:read` check and why a failed count does not fail the list
- [x] ~~`CLAUDE.md` updated~~ (N/A: no new project-wide rule)

## Project Documentation

- [x] `GUIDE-data-entry.md` (generates `docs/data-entry.md`): new "Counts on
cards" section with config, labels, ACL behaviour and the kanban-only rule
- [x] `docs/data-entry/api-reference.md`: new "Comment counts on list rows"
section for `comment_counts` / `_comment_count`
- [x] ~~`README.md`~~ (N/A: no top-level feature list change)

## External Documentation

- [x] ~~Tool README~~ (N/A: no external tool touched)
- [x] ~~Migration notes~~ (N/A: additive config keys and an opt-in query
parameter)

## Verification

- [x] The documented config was run: the e2e board `feature-counts` uses it
- [x] `just docs` regenerated `docs/data-entry.md`; markdownlint clean
- [x] The documented absent-not-zero and fail-soft behaviour is pinned by
handler tests
