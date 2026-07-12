---
id: TKT-PF4E6S
type: ticket
title: Font tokens in _rela.css so apps inherit host typography
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Add typography tokens (`--font-family`, `--font-size-{sm,base,lg,xl}`) to the
`_rela.css` served to custom apps, plus an `html` rule that applies the font
family + base size. Without this, an app that links `_rela.css` for the color
tokens still falls back to the browser default serif at 16px, so its text does
not match the host UI.

This is the typography sibling of the color palette (FEAT-4OEJ): the token
*names* are the frozen app-facing contract; web serves SPA-matching values and
the native client serves its own system-font values.

Emitted from an always-present `appTypographyCSS` block (independent of the
per-project palette introduced by the palette feature), so linking apps get the
font tokens whether or not a resolved palette is present.

## Acceptance criteria

1. `_rela.css` served at `/api/v1/_apps/<id>/_rela.css` includes `--font-family`
and the `--font-size-*` scale, plus an `html { font-family; font-size }` rule.
2. The tokens are emitted on both the nil-palette and resolved-palette paths.
3. Existing color tokens and `.btn`/`.input`/`.card` controls are unchanged.
4. Docs (data-entry guide) note that `_rela.css` now also provides typography.
