---
id: DOCS-AH7A40
type: docs-checklist
title: 'Docs: field span layout (12-column grid) for views and forms'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc/comments on new exported symbols
- [x] Non-obvious decisions explained with WHY, not WHAT

`SpanColumns` and `validateSpan` carry the rationale for being loud rather than
clamping. `properties-list.css` documents why it exists (three forked scoped
copies) and marks the row-behaviour rules as deliberate so nobody "fixes" the
empty remainder. `fieldSpan.ts` explains why it returns `undefined` rather than
`12` — the default belongs to the CSS fallback, in one place.

## Project Documentation

- [x] `frontend/CLAUDE.md` updated
- [x] ~~CLAUDE.md (root)~~ (N/A: frontend-scoped convention)

Added "Property layout: one grid, one stylesheet": don't redefine the shared
classes, the default lives only in the `var(--field-span, 12)` fallback,
`fieldSpan.ts` is the single config→CSS boundary, and an explicit warning about
the equal-specificity trap in `DynamicForm` that swallowed every authored span
the first time.

## External / User-Facing Documentation

- [x] `docs/data-entry.md` — **required**, this PR adds a public config key.

New "Field Layout (`span`)" section under Field Options, plus a `span` row in
the options table. Documents the 12-column grid, the full-width default, a
worked example, and all four row-behaviour rules (unfilled remainder stays
empty, non-fitting fields wrap, narrow screens collapse, long-form values ignore
span).

Frames the model in terms of *why*: adjacency is declared, because a layout that
regroups itself at different window sizes is not one you can reason about.

- [x] ~~docs/metamodel.md, docs/cli-reference.md, README.md~~ (N/A: no
metamodel, CLI or project-level change.)

## Verification

- [x] Documentation matches the implemented behaviour
- [x] Examples in docs actually work

**The documented error message was verified byte-exact against real output**
rather than transcribed: a throwaway test printed `validateSpan(13, ...)` and
the string matches the docs character for character. A docs example that has
drifted from the actual message is worse than none — it sends the reader looking
for text that never appears.

The worked YAML example is the one now living in the prototype project, so it is
exercised by the running demo rather than only existing in prose.

`markdownlint-cli2` clean on both files.
