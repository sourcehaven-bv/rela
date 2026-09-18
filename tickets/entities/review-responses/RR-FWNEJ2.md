---
id: RR-FWNEJ2
type: review-response
title: Rebase onto develop validated the extraction by accident
finding: 'While this ticket was in flight, develop gained a task-list command (a new taskListItem.ts module,
  a new entry in the shared BLOCK_COMMANDS catalogue, a new nodeWhere probe kind, and a new toolbar glyph).
  The rebase conflicted in three files: BlockIcon.vue, markdown-content.css and relaEditorTheme.css. Two
  of those conflicts were develop editing EasyMDE rules this change deletes outright, but markdown-content.css
  also carried new .md-table-scroll rules that taking either side wholesale would have silently dropped.'
severity: minor
resolution: 'Resolved by rebuilding markdown-content.css from develop''s version minus the .editor-preview
  block, rather than taking either side, so the nine new .md-table-scroll rules survive. develop''s new
  taskList glyph moved into the shared editorIcons.ts table.


  Worth recording because it is unplanned evidence for the ticket''s whole premise. A command added to
  the shared catalogue by someone who had never seen this branch appeared in the sandboxed app editor
  with no code change: the toolbar builds from the catalogue, the glyph test demands one per command,
  and the availability probe covers the new nodeWhere kind. Under the forked arrangement this replaces,
  the app editor would simply have lacked the button, and nothing would have said so. After the rebase,
  2875 frontend tests pass — including develop''s new task-list tests running against the extracted modules.'
status: addressed
---
