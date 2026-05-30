---
id: TKT-GUPMK
type: ticket
title: Inline-edit the full markdown content body with autosave
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Goal\n\nExtend the inline-edit plumbing to the `content` display mode so the whole markdown body of an entity can be edited inline with autosave-on-blur. This is the 'Today's Notes' textarea in the Daily Notes mockup.\n\n## Scope\n\n- A `markdown` widget that renders read-only markdown in display mode and a textarea (or similar) in inline-edit mode.\n- Autosave on blur uses the same `useInlineEdit` composable from ticket 3.\n- The existing checkbox-in-markdown toggling continues to work alongside body edits.\n- Optional 'Version History' affordance below the editor (existing git-backed feature, just needs a rendering hook).\n- View config: `display: content` accepts `editable: true`.\n\n## Non-goals\n\n- No rich-text WYSIWYG. Plain markdown textarea is sufficient for the first cut.\n- No collaborative editing.\n- No live preview toggle.\n\n## Why\n\nLast missing piece for the Daily Notes screen (and any screen with a free-text notes body).
