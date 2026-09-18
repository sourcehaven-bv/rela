---
id: DOCS-P5QN1O
type: docs-checklist
title: 'Docs: Markdown table rendering — per-table scroll and cell bounds'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc/docblocks on new exported symbols
- [x] Non-obvious decisions explained with rationale
- [x] Load-bearing constraints recorded where they can be found

`wrapTablesForScroll` and `tableLabel` carry docblocks stating **why**, not just
what: why the transform runs after sanitization (string-splicing around
DOMPurify output is how this kind of helper becomes a bypass), why `<template>`
is used (inert parsing — no requests, no script execution), and why the
`role`/`tabindex`/label trio is required rather than decorative.

The CSS comment in `markdown-content.css` is the main deliverable here. It
replaces a comment that **asserted something false** — the old text claimed wide
tables scroll, which measurement disproved. The new comment records:

- the root cause (`overflow-wrap: anywhere` counts toward min-content per CSS
Text 3 §5.5; `break-word` does not),
- that all three declarations are load-bearing, with the specific failure each
one prevents, including the measured 2111px blow-out if `max-width` is dropped,
- why the table keeps `display: table` (a11y roles, `border-collapse`,
RR-5ZVPC5's narrow-table fill),
- why Milkdown is excluded.

This matters more than usual because the previous ticket's comment is exactly
what made the bug survive: a confident, wrong comment stopped anyone
re-checking.

## Project Documentation

- [x] ~~docs/metamodel.md~~ (N/A: no metamodel change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~docs/data-entry.md~~ (N/A: no user-facing behaviour or configuration
change — markdown tables render as they always should have. There is no setting
to document and no new authoring affordance.)
- [x] ~~CLAUDE.md / frontend/CLAUDE.md~~ (N/A: introduces no new pattern. The
change *follows* existing documented patterns — the shared `.md-body`
stylesheet, the post-render DOM transform already used for PlantUML, and the
mandated two-shadow focus-ring token pair.)
- [x] ~~README.md~~ (N/A: not project-level)

## External Documentation

- [x] ~~Release notes / migration guide~~ (N/A: a rendering fix with no
migration step. Existing content renders better; no author action required.)

## Rationale

The durable documentation for this change is the code comment, not a doc page. A
reader hitting this CSS will be someone tempted to simplify it — and the three
bounds look arbitrary until you know what each one prevents. That knowledge
belongs at the declaration site, which is where it now lives.
