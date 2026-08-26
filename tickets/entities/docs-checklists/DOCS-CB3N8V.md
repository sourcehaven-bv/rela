---
id: DOCS-CB3N8V
type: docs-checklist
title: 'Documentation: CheckboxWidget is unstyled — the only widget with no design tokens'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] ~~Function/type docs if public API~~ (N/A: no functions or types added —
      the change is scoped CSS plus tests)

Three comments carry non-obvious reasoning that the CSS cannot state itself:

- **Why `:not(.display-checkbox)` instead of a plain override.** The specificity
  arithmetic ((0,3,1) vs (0,3,0), with Vue's `[data-v-x]` lifting both equally)
  is the whole reason the first attempt silently failed. Written at the rule so
  the next editor does not "simplify" it back into the bug.
- **Why `color-mix` instead of the siblings' literal.** Distinguishes a
  fallback that never renders from a literal that always does — without it, the
  line looks gratuitously different from its neighbours.
- **Why 0.6 and why the siblings' `--hover-bg` does not transfer.** On a
  checkbox the background *is* the checked signal, so the sibling pattern would
  erase state to convey read-only.

The test file additionally records what the hook test does **not** cover, so
its guarantee is not overread — that overreading is what let RR-CBC1XZ ship.

## Project Documentation

- [x] ~~README updated~~ (N/A: no user-visible feature, command, or config)
- [x] ~~CLAUDE.md updated~~ (N/A: introduces no new pattern — the change
      *adopts* existing documented ones: `scales.css` tokens and the
      `properties-list.css` precedent on shared styles)
- [x] ~~Help text accurate~~ (N/A: no CLI surface)

Deliberately **not** documented in `frontend/CLAUDE.md`: the forced-colors
block and the `color-mix` ring are currently one widget's local fixes, not
conventions. Writing them up as house style would assert a consistency the rest
of the widget set does not yet have (`forced-colors` appears nowhere else in
`src/`). If [[RR-CBLEV8]]'s shared `.rela-checkbox` lands, that is the point at
which they become a documented pattern.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG — releases are
      generated from commit history, and the commit message carries the why)
- [x] ~~API docs updated~~ (N/A: no API change. `docs/data-entry.md` documents
      the `widget:` override and the widget set; which widget the registry
      selects, and every wire type, are unchanged — only how an already-selected
      checkbox is painted)

Verified rather than assumed. `docs/data-entry.md` is generated from
`docs-project/entities/guides/GUIDE-data-entry.md`, and it *does* mention
checkboxes — five times. Each was read: the widget table row ("Toggle
checkbox", boolean properties), the type→widget defaults prose, a `widget:
checkbox` config example, and the filter-comparison note that a checkbox `true`
matches both the boolean and the string. All describe **which widget renders a
boolean and how its value behaves** — none describes how the control is
painted. Unchanged by this ticket, so there is nothing to regenerate.
