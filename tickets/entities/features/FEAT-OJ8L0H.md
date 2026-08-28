---
id: FEAT-OJ8L0H
type: feature
title: Visual design system for the data-entry SPA (tokens, icons, layout primitives)
description: 'A coherent internal design system for the data-entry SPA: design tokens beyond color (spacing, radius, typography, elevation), a real SVG icon set replacing emoji, and shared layout primitives so detail/list/form/kanban surfaces present data consistently and scannably.'
status: proposed
---

## Summary

The data-entry SPA has a well-maintained *color* token layer
(`frontend/src/styles/tokens.css`, shared verbatim with the Go binary as the
app-facing `_rela.css` contract) but no equivalent discipline for anything else.
The result is measurable drift: 7 border-radius values, 18 font sizes, spacing
that mixes px and rem, emoji standing in for an icon system, and per-view field
layouts that were each tuned by hand.

This feature is the umbrella for making the SPA's visual language systematic:

- **Tokens beyond color** — spacing, radius, typography, elevation scales,
reconciled with the `--font-size-*` names already frozen into the app contract
by TKT-PF4E6S.
- **A real icon set** — self-hosted, tree-shakeable, `currentColor`-driven SVG
replacing emoji glyphs, so icons theme correctly and render identically across
operating systems.
- **Layout primitives** — shared patterns for presenting entity properties
(label/value rhythm, content-proportional widths, read vs. edit surfaces) so
scannability is a property of the system rather than of each view.

Distinct from FEAT-4OEJ, which covers *user-configurable* palettes: this is
about the internal consistency of the shipped UI. The two meet at `tokens.css`,
so token names introduced here must not break the app contract.
