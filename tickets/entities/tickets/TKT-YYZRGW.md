---
id: TKT-YYZRGW
type: ticket
title: Consistent markdown styling across data-entry render surfaces
kind: enhancement
priority: medium
effort: s
status: done
---

# Consistent markdown styling across data-entry render surfaces

## Problem

Rendered markdown looked inconsistent and sometimes broken across the four
surfaces that display it in the data-entry SPA:

- **Tables** in entity content bodies had no borders/padding at all (the
original report).
- **Blockquotes** (`> text`) were unstyled in the EasyMDE live-preview pane.
- **Lists** rendered with wrong/missing markers in the preview.
- The EasyMDE preview used the library's hardcoded light-theme defaults
(borderless `#ddd`/5px tables, no header shading), so it didn't reflect what the
entity view actually renders.

Root cause: each Vue component (`EntityDetail`, `DocumentView`,
`DocumentsPanel`) kept its **own copy** of markdown element CSS, and they had
drifted (h1 24px vs 28px; EntityDetail had no blockquote/pre/hr/img rules at
all). The EasyMDE preview had no re-skin.

## Solution

Single source of truth: `frontend/src/styles/markdown-content.css` (loaded once
from `main.ts`), keyed on:
- `.md-body` — the three v-html surfaces (all now carry the class).
- `.EasyMDEContainer .editor-preview` — the EasyMDE live preview (specificity
beats the library's bundled defaults regardless of load order).

Covers every element: headings (h1–h6), p, ul/ol/li (with nested
disc→circle→square cascade + restored markers), task-list checkboxes (bullet
removed, aligned; handles both tight `<li><input>` and loose `<li><p><input>`
shapes), code, pre, blockquote, hr, img, links, tables, kbd. All colors from
theme tokens → light + dark for free.

The per-component CSS blocks were removed (net −130 lines). The sandboxed
app-editor bundle (`app-editor/relaEditorTheme.css`) inlines its own CSS and
can't load the global sheet, so the `.editor-preview` rules are mirrored there —
and a new test (`markdownContentMirror.test.ts`) asserts the two copies stay
identical, so the drift this PR fixes can't silently return.

## Acceptance criteria

1. Tables, blockquotes, lists, code, task-lists render styled and consistent
across EntityDetail, DocumentView, DocumentsPanel, and the EasyMDE preview.
2. Wide tables scroll horizontally; long unbroken tokens wrap (no body-wide
scrollbar).
3. All styling from one shared source; light + dark via tokens.
4. A test fails CI if the app-editor mirror drifts from the shared sheet.

## Verification

- `npm run typecheck` clean; `npm run test:run` 73 files / 1163 tests pass
(incl. new GFM-table structural test + mirror sync test); `npm run lint` 0
errors; `npm run build` succeeds.
- Puppeteer visual + computed-style verification against a live server on the
entity view and the EasyMDE side-by-side preview: borders, header shading, list
markers, nested cascade, task-list bullet removal, blockquote accent bar,
long-URL wrap (no horizontal overflow) all confirmed in dark mode; theming is
token-driven so light mode follows by construction.

Subsumes follow-up TKT-W3OPRX (consolidate remaining triplicated markdown CSS).
