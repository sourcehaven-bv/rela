---
id: RES-AV732E
type: research
title: Which WYSIWYG editor can edit rela's markdown without corrupting it?
summary: Surveyed EasyMDE, Tiptap, Crepe and Milkdown for WYSIWYG markdown editing; chose Milkdown because remark serialization can be pinned, verified lossless over 3,939 corpus files (0 semantic drift).
status: done
---

## Problem

rela stores entity bodies as markdown files in git. The data-entry form edits
them with EasyMDE, which is a source editor: the author sees raw markdown. That
makes an entity reference an opaque code span (`` `TKT-007` ``) rather than the
thing it points at, and it means the live preview is a second rendering surface
to keep in sync with the entity view.

Moving to WYSIWYG means the editor parses every body it opens and re-serializes
it on save. **That is the risk.** The files are version-controlled and
human-authored; an editor that reformats on open would put a diff in git for
every entity someone merely looked at, and one that drops a construct it does
not model would silently destroy content.

So the question is not "which editor looks best" but "which editor can be
trusted to write back what it read".

## Context

- Bodies are commonmark + GFM (tables, task lists, strikethrough), with code
spans carrying entity IDs.
- The corpus in this repo is 3,939 entity files, which is a usable population
to test against rather than reason about.
- `styles/markdown-content.css ` is already the single source of truth for how
rendered markdown looks (TKT-W3OPRX). Whatever edits should inherit it.
- The sandboxed app editor has a stricter CSP (no `'unsafe-inline' ` on
`style-src `), so anything relying on inline styles is a problem there.

## Options

### 1. Keep EasyMDE

*Pros*: zero risk to stored bytes — it never reparses; already integrated.

*Cons*: cannot render a reference as its title, which is the reason for the
change. The preview duplicates rendering logic. CodeMirror 5 underneath.

*Effort*: none.

### 2. Tiptap

*Pros*: ProseMirror-based, large ecosystem, good docs.

*Cons*: markdown is not its native representation — it is HTML/JSON first, with
markdown via a community extension. For a system whose storage format IS
markdown, that puts a lossy translation in the middle of the write path.

*Effort*: m.

### 3. @milkdown/crepe

*Pros*: batteries included, looks good immediately, Notion-like out of the box.

*Cons*: ships its own theme, which fights the existing token system; does not
expose the remark-stringify options needed to pin serialization. Opinionated
chrome would have to be undone.

*Effort*: s to start, then fighting it.

### 4. Milkdown (core + kit) — RECOMMENDED

*Pros*: ProseMirror for editing, **remark for serialization** — the same mdast
pipeline the rest of rela reasons about. `remarkStringifyOptionsCtx ` allows
pinning the output format exactly. Custom nodes (`$node `) make an entity
reference a real atomic node with a declared markdown round trip. `@milkdown/kit
` bundles every plugin needed (commonmark, gfm, history, listener, slash, block,
cursor, trailing), so it is one dependency.

*Cons*: more assembly than Crepe — toolbar, menus and theming are ours to write.
Serialization must be pinned deliberately or it reformats.

*Effort*: l.

## Recommendation

**Milkdown**, with a hard gate before any UI work: pin the stringify options,
then run the entire corpus through parse → serialize and measure.

The measurement has to distinguish two failures that look the same in a diff:

- **Byte churn** — output differs, meaning identical (table padding, bullet
style, escaping). Unavoidable for any serializer; must be *suppressed* at
write-back rather than eliminated.
- **Semantic drift** — output means something different. Must be zero, and is
not something a guard can paper over.

Result over 3,939 files: **1,732 with byte churn, 0 semantic drift, 0
non-idempotent.** Churn is absorbed by `guardWriteBack `, which returns the
original bytes when the semantic projection is unchanged and refuses to write at
all when it is not.

Bundle cost was measured rather than estimated by building both variants: JS
+128KB, CSS −43KB, fonts −332KB — **net −247KB**, because Milkdown's icons are
inline SVG and Font Awesome 4.7 can go.

## Consequences

- Serialization is a pinned contract (`RELA_STRINGIFY_OPTIONS `) with a corpus
test in CI. Changing it is a deliberate act with a visible gate.
- The editor inherits `.md-body ` instead of restyling, so the editing surface
and the entity view cannot drift apart.
- The app editor stays on EasyMDE for now (TKT-D2JML7); the CSP question there
is unresolved and does not need to block this.
