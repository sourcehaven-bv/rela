---
id: DOCS-4HD23U
type: docs-checklist
title: 'Documentation: Style HTML comments as muted chips in the Milkdown editor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`commentNode.ts` carries a file-level doc explaining why a separate node exists
rather than CSS on the preset's `html` node, and why the remark retype (not
`parseMarkdown`) is what wins the match. Three non-obvious decisions are
documented at their site: the regex's exclusion of an interior `-->` (with the
backtracking failure that motivated it), the hand-rolled traversal instead of
`unist-util-visit`, and the `createTextNode` security property in `toDOM`.
`commentBody` and `isCommentNode` are exported and carry doc comments; the
former documents its nil behaviour.

## Project Documentation

- [x] CLAUDE.md updated (if new patterns)
- [x] ~~README updated~~ (N/A: no project-level change; the editor's rules live
in `frontend/CLAUDE.md`)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

`frontend/CLAUDE.md` gained a comment-node section in the markdown-editor part,
stating the three rules a future change must not break: the retype is what wins
the match, the matcher is an anchored allowlist so unrecognised markup stays
visible, and `toMarkdown` writes original bytes rather than the trimmed label.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo has no CHANGELOG.md)
- [x] ~~API docs updated~~ (N/A: no API surface change — this is editor chrome;
the stored markdown format is unchanged by design)

`docs/data-entry.md` was assessed and deliberately not updated: it documents
data-entry features, and this is editor rendering of content that already
existed. Nothing an operator configures or a user invokes has changed.
