---
id: TKT-8TVDKI
type: ticket
title: Insert and edit external links in the Milkdown editor (plus horizontal rule and undo/redo)
kind: enhancement
priority: medium
effort: m
status: review
---

The WYSIWYG editor has no way to create, retarget or remove a hyperlink. Add a
link command with a floating edit tooltip, restricted to http/https/mailto, and
close two smaller gaps found alongside it: no horizontal-rule affordance and no
undo/redo buttons.

## Problem

The Milkdown editor in data-entry forms cannot create, retarget or remove a
hyperlink. The `link` mark exists in the schema because the commonmark preset
provides it, so a link already present in an entity body parses, renders as an
`<a>`, and round-trips safely. But nothing in the UI can produce one:

- `INLINE_COMMANDS` (`editorCommands.ts:82`) has four entries — bold, italic,
strikethrough, inline code. No link.
- No toolbar button, and `BlockIcon.vue` has no link glyph.
- No link tooltip. `@milkdown/plugin-tooltip` and `@milkdown/components` (which
ships `linkTooltipPlugin`) are installed via `@milkdown/kit` but never imported.
- No paste handling anywhere in `frontend/src`, so pasting a URL over a
selection produces plain text.
- No input rule for `[text](url)`, so typing the markdown by hand does nothing.

The old EasyMDE editor this replaced had a link button
(`src/app-editor/relaEditor.ts:193`, still present in the sandboxed app editor),
so this is a parity regression from TKT-3I9DDY.

Two smaller gaps were found in the same sweep and are cheap to close alongside:

- **Horizontal rule** — the serializer already writes `---`
(`serializerContract.ts:41`), only the insert affordance is missing.
- **Undo/redo buttons** — the `history` plugin is loaded and binds its own
keymap, but there is no visible control.

## Scope

In scope:

1. Insert a link over the current selection, and over an empty selection
(prompting for the link text).
2. Edit an existing link's URL.
3. Remove a link, keeping the text.
4. Pasting a URL while text is selected wraps that text in a link.
5. A floating tooltip on the link showing the URL with edit and unlink actions.
6. ~~`Mod-k` opens the same UI as the toolbar button.~~ (descoped in planning:
already bound to the command palette)
7. Horizontal-rule toolbar command.
8. Undo and redo toolbar buttons.

Out of scope: image insertion (there is no upload path in the editor, so it
would be URL-only and is a separate decision), headings 4-6, the `/` block menu
that `filterBlockCommands` was written for but never wired up, entity targets in
the link dialog (the `@`-mention and entity-ref button already cover internal
links and use a different markdown construct with ACL-gated titles), link
`title` attributes, reference-style links, and autolink literals.

## URL policy

Accept `http:`, `https:` and `mailto:` only. Refuse `javascript:`, `data:`, any
other scheme, and relative or scheme-less paths. A bare host such as
`example.com` gets `https://` prepended rather than being refused.

This is enforced at insert time, in addition to whatever the renderer does.
Entity bodies are rendered on several surfaces, so a stored `javascript:` URL is
a real XSS surface and the editor should not be the component that admits one.
`rawHtmlPassthrough.test.ts:91` already asserts the preset strips such an href
on render; this adds the write-side half.

## Design notes

Follow the house convention documented in `editorCommands.ts`: commands are
named by their registered **slice name string**, never `$Command.key`. A new
toolbar entry is an `EditorCommand` descriptor plus a `BlockIcon.vue` glyph case
keyed on the command `id`; the toolbar picks it up through its `v-for`, active
state comes from `probe`, and disabled state from the `commandAvailability.ts`
dry run.

The link tooltip has no precedent in this directory. The closest existing shapes
are the `@` mention menu (`useMentionMenu.ts` + `MentionMenu.vue` + a
`SlashProvider`) for a floating surface driven by editor state, and
`EntityPickerModal` for arbitrary user input inserted via a direct
`view.dispatch`. Whether to adopt `@milkdown/components`' `linkTooltipPlugin` or
build one in the local idiom is the main open question for planning — the
packaged one brings its own styling and English strings, which may fight
`milkdownEditor.css` and the `.md-body` contract.

Undo/redo buttons need care on two points: their availability must come from the
history plugin's own `undoDepth`/`redoDepth` rather than a guess, and they must
not be probed for active state.

## Acceptance criteria

1. With text selected, the link button inserts a link; the body serializes to
`[text](https://example.com)`.
2. With the cursor inside a link, a tooltip appears showing the URL, with edit
and unlink actions.
3. Unlink removes the mark and keeps the text.
4. `javascript:alert(1)` and `data:text/html,x` are refused with a visible
message; no link is created AND nothing is written to the mark.
5. `example.com` is stored as `https://example.com`.
6. Pasting `https://example.com` over a selection wraps it rather than replacing
it.
7. The horizontal-rule command inserts a break serializing to `---`.
8. Undo and redo buttons work and are `aria-disabled` (never natively
`disabled`) at the ends of the history stack.
9. Opening an entity that contains a link and saving it without edits emits
nothing (the write-back guard still holds).
10. A link can be inserted, retargeted AND unlinked without a mouse.
11. Selecting across an existing link and pressing the link button retargets it
— it never destroys the link or discards the entered URL.
12. A pre-existing `javascript:` link round-trips unchanged when an unrelated
part of that body is edited.
13. `mailto:a@b.com?bcc=x@y.com` is stored as `mailto:a@b.com`.
14. `example.com:8080/path` is stored as `https://example.com:8080/path`.

Note: `Mod-k` was descoped during planning — it is already bound to the command
palette. See PLAN-RY25IP.
