---
id: FEAT-014
status: implemented
summary: WYSIWYG markdown editor (Milkdown/ProseMirror) for entity content, in data-entry forms and in sandboxed custom apps
description: Milkdown WYSIWYG markdown editor with a formatting toolbar, table controls and `@` entity-reference completion, used by data-entry forms and by the `<rela-editor>` element custom apps embed
title: Markdown editor for entity content
type: feature
---

Entity body content is edited in a WYSIWYG markdown editor built on Milkdown
(ProseMirror). It parses the body it opens and re-serializes it on save, so the
editing surface IS the rendered view rather than a source buffer beside a
preview pane.

There are two surfaces, running the same editor:

- the data-entry form in the SPA (`frontend/src/components/forms/milkdown/`)
- `<rela-editor>`, the Custom Element sandboxed custom apps embed
  (`frontend/src/app-editor/`), served at the reserved per-app path
  `_rela-editor.js`

Features:

- Formatting toolbar: bold, italic, strikethrough, inline code, headings,
  lists, quote, code block, table
- Buttons reflect the formatting at the cursor, and disable where a command
  would do nothing (a heading inside a list item, say)
- Table controls: insert and delete rows and columns, shown only inside a table
- `@` completion for entity references, which are stored as code spans
- Checklist support

Two properties are load-bearing rather than incidental:

- **Opening an entity and saving it emits nothing.** Serialization is pinned and
  checked against every entity body in the repository; the reformatting that
  survives is suppressed rather than written back.
- **Entity-reference titles are ACL output.** The form resolves them only from
  the server's per-principal mentions map. The custom-app editor has no such
  endpoint available to it, so it renders bare IDs rather than deriving a title
  from the graph.

Superseded the EasyMDE/CodeMirror editor: the form in TKT-3I9DDY, the
custom-app element in TKT-D2JML7.
