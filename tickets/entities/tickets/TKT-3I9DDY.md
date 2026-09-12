---
id: TKT-3I9DDY
type: ticket
title: Replace EasyMDE with Milkdown (ProseMirror) in data-entry forms
kind: enhancement
priority: medium
effort: l
status: planning
---

## Description

Replace the EasyMDE markdown editor in the data-entry SPA with Milkdown, a
ProseMirror-based WYSIWYG editor.

EasyMDE edits markdown as source text in a CodeMirror buffer. Entity references
are raw code spans, so an author sees `TKT-007` rather than the title it points
at, and the live preview is a second rendering surface that has to be kept in
visual sync with the entity view. Milkdown edits the parsed document, so a
reference can render as a real titled link in place and the editing surface IS
the rendered view.

### Scope

Phases 0-3. The standalone `<rela-editor>` custom element for sandboxed apps
(TKT-D2JML7) stays on EasyMDE and is explicitly out of scope; the app-editor
bundle must keep building unchanged.

- **Phase 0** — pin serialization (`RELA_STRINGIFY_OPTIONS`) and prove a
round trip is semantically lossless across the whole entity corpus.
- **Phase 1** — the editor itself: `entityRef` node, resolution plugin,
write-back guard.
- **Phase 2** — `@` completion menu for inserting references inline.
- **Phase 3** — server-side `mentions` on single-entity GET so titles resolve
through the read gate.

### Why round-trip fidelity is the hard part

A WYSIWYG editor round-trips every body it opens, so opening an entity and
saving it without edits must emit nothing. Two distinct problems:

- **Byte churn** — the serializer reformats without changing meaning (table
cell padding, bullet style). Suppressed by `guardWriteBack`, which compares a
semantic projection of the mdast and returns the original bytes.
- **Semantic drift** — the serializer changes what the document MEANS. Not
suppressible; the corpus test gates it at zero.

Measured over 3,939 corpus files: 1,732 with byte churn, **0 semantic drift, 0
non-idempotent**.

### Display contract

The title is what matters and shows everywhere. The ID is optional and, if shown
at all, appears only in the editor (the completion menu, to disambiguate two
entities with the same title). Stored markdown holds the bare code span and
never the title.

### Security

Entity-reference titles are load-bearing for ACL (BUG-R9EHKV). Mentions route
through `visibility.Reader.Filter`: an entity the principal may not read
produces no mention at all, and a readable entity whose display-title property
is redacted falls back to its ID with `inaccessible: true`. The editor never
derives a title from the graph itself — it only renders what the server sent.
