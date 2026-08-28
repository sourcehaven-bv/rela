---
id: DOCS-FR6K1P
type: docs-checklist
title: 'Documentation: Focus rings are a hardcoded indigo that ignores the theme, and vanish entirely in High Contrast'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] ~~Function/type docs if public API~~ (N/A: no exported Go API added; the
      one Go change is three map entries in an existing unexported function,
      whose doc comment was extended)

Four comments carry reasoning that the code cannot state itself, each written
at the site where the mistake would be repeated:

- **`tokens.css`** — why the ring is opaque (every translucent value fails WCAG
  2.2 §1.4.11; the old one measured 1.13:1), and why `--focus-ring-gap` exists
  (a ring flush against its own accent border is 1:1 contrast).
- **`focus-ring.css`** — why `!important` is warranted, which is otherwise
  banned. Without it a bare `:focus-visible` (0,1,0) loses to every
  `input:focus { outline: none }` (0,1,1) and the rule does nothing.
- **`deriveTheme`** — that it is the SECOND renderer of the token contract, so
  a token added to `tokens.css` alone silently does not exist for
  palette-configured projects. This is the [[RR-FRC4PL]] trap.
- **`focusRing.test.ts`** — why the guards read source rather than mounting
  components, and (honestly) what they still cannot see.

## Project Documentation

- [x] README updated (if applicable) — N/A, no project-level change
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate~~ (N/A: no CLI surface)

`frontend/CLAUDE.md` gains a "Focus rings are two shadows and a token" section
next to the existing design-token rules: the canonical snippet, the three
things it encodes (a literal cannot follow the theme; the ring is opaque on
purpose; the gap band is load-bearing), the `!important` exception and why it
is the only sanctioned one, and a pointer to the guard test.

This was a real requirement rather than a formality — the ticket adds three
names to a published contract, and without the written rule the next component
author hand-writes another literal. The guard only catches that if they happen
to pick a colour it knows.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: repo keeps no CHANGELOG; releases are
      generated from commit history)
- [x] API docs updated (if applicable)

`docs/customisation.md` is the operator-facing surface and did need changing:
`--accent-color` now reaches further than the obviously-accented elements,
because focus rings derive from it. Documented, with the two constraints an
operator needs before overriding `--focus-ring` directly (keep it opaque or it
fails contrast; keep the gap distinct or the ring merges into the border), and
the note that forced-colors indication is deliberately not overridable — it is
an accessibility floor, not a skin.

Verified rather than assumed: `docs/customisation.md` is NOT generated (no
entry in `scripts/generate-docs.sh`, no source guide under
`docs-project/entities/`), so editing it directly is correct. `docs/data-entry.md`
IS generated and was deliberately not touched — it documents config surfaces,
none of which changed.
