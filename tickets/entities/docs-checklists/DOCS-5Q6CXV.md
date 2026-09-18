---
id: DOCS-5Q6CXV
type: docs-checklist
title: Documentation
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

The load-bearing comments, each recording a decision that is not visible from
the code:

- `relaEditor.ts` header: the six-item contract, why light DOM rather than
shadow DOM, which modules are shared with the SPA editor and why, and the one
deliberate difference (bare IDs, no titles).
- The `value` getter: what the write-back guard does and does NOT guarantee —
an unedited body is byte-identical, an edited one is fully reserialized — plus a
pointer to where that is documented for app authors.
- The `input` event: why it comes off the ProseMirror view hook rather than
either the markdown listener (debounced 200ms) or `appendTransaction` (runs
before the new state exists, so listeners read the PREVIOUS document).
- `_onEditorFocus`: why focus is tracked, since every toolbar command refocuses
and re-snapshotting there swallowed the `change` event entirely.
- `_withdrawPromptedRef`: why the text is withdrawn but the dirty flag is not.
- `mentionMenuState.ts` and `editorPreset.ts` headers: why each exists as a
shared module rather than a copy, naming the drift it prevents.
- `vite.editor.config.ts`: why the ProseMirror CSS is resolved through
`@milkdown/prose` rather than the `@milkdown/kit` re-export shims (the shims are
bare `@import`s a browser silently drops), and what each build guard catches.

## Project Documentation

- [x] README updated (if applicable) — N/A: no project-level surface change.
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes) — N/A: no CLI change.

`internal/dataentry/CLAUDE.md`: rewrote the `<rela-editor>` entry. It previously
recorded a third served asset (`_rela-editor.woff2`) that no longer exists, said
the editor was EasyMDE/CodeMirror, and gave CM5-in-a-shadow-root as the reason
for light DOM. It also carried the CSP claim this ticket disproved, which is now
corrected with the reasoning (`style-src` governs stylesheets and the `style`
ATTRIBUTE; ProseMirror writes DOM style PROPERTIES, which is the CSSOM).

`frontend/CLAUDE.md`: added the app-editor section — the share-don't-mirror rule
and what may not be imported into a plain IIFE, the bare-ID constraint, why
`input` comes off the view hook, and why `.value` reads through the guard.
Corrected the `scales.css` row of the token-contract table, which said
"SPA-only" and is now also concatenated (re-anchored) into the editor bundle.

## External Documentation

- [x] Changelog entry added — N/A: this project keeps no changelog.
- [x] API docs updated (if applicable)

`docs-project/entities/guides/GUIDE-data-entry.md` gained a section documenting
`<rela-editor>` for app authors. It was not documented at all before, which was
a gap from TKT-5F9V56: a public opt-in surface with a stable contract that only
appeared in an internal CLAUDE.md.

The section covers how to embed it, the six-item API as a table, a worked
load-and-save example against the bridge, and the three behaviours an author can
otherwise only discover from a diff:

- setting `value` from code fires no `input` (so loading content cannot start an
autosave loop),
- an unedited body comes back untouched, but any edit reserializes the whole
body to normal form,
- entity references show as plain IDs here, with the reason (titles are
per-principal and only the server can decide them).

Written from the app author's side rather than the implementer's: it says which
editor is underneath is an implementation detail that has already changed once,
and that apps written against the six-item API needed no edit when it did.

`FEAT-014` was also rewritten. It described "EasyMDE markdown editor with
toolbar, preview, and fullscreen editing" — software that no longer exists
anywhere in the repo, since TKT-3I9DDY replaced the form editor and this ticket
replaced the other one. It now describes the Milkdown editor and both surfaces
that run it.
