---
id: RR-CBS3AC
type: review-response
title: 'appearance:none + outline:none erases the control in forced-colors (Windows High Contrast) mode'
finding: 'In `forced-colors: active` the OS overrides background-color and border-color with system colours, drops `box-shadow` entirely, and does not guarantee `::after` pseudo-element rendering. The checked state was communicated ONLY by `background: var(--accent-color)` plus the `::after` checkmark, and the focus indicator ONLY by box-shadow (with `outline: none` removing the native one). Net effect for a High Contrast user: no visible focus ring, and potentially no checked/unchecked distinction. Before this change the native control was immune — the OS drew it correctly for free — so the change materially worsens this one control. `forced-colors` appears nowhere in src/, so this is a codebase-wide gap rather than a deviation from a local pattern.'
severity: significant
resolution: 'Added a `@media (forced-colors: active)` block that sets `appearance: auto` — handing the control back to the OS in exactly the mode where the OS draws it better than we can — and restores a real `outline: 2px solid Highlight` focus ring, using the system Highlight colour keyword.'
status: addressed
---

Worth recording the asymmetry: the same exposure exists in
`RelationCards.vue`'s `.inline-edit-checkbox`, which shipped without it. This
change does not fix that instance, but it also does not copy the gap forward —
the widget is now the better-behaved of the two. Consolidating them is
[[RR-CBLEV8]].

`:focus-visible` (rather than `:focus`) was already correct — no ring on mouse
click, ring on keyboard focus — and is retained.
