---
id: RR-BRVI7N
type: review-response
title: CommandPalette renders an empty invisible li when the href is unresolvable
finding: 'CommandPaletteModal.vue:288-309 — the <li role=option> renders unconditionally from v-for but v-if guards only the inner RouterLink, and all three text spans live inside it. When entityDetailHref returns '''' the option renders as an empty zero-height <li>: the visible row count disagrees with the result count, arrow-key highlight appears to skip an invisible option, scrollHighlightedIntoView targets a zero-height element, and a screen reader announces an option with no accessible name. Worse than the pre-change behaviour, where such a row at least rendered its text and was merely inert.'
severity: critical
resolution: 'Fixed in CommandPaletteModal.vue: the three text spans now render in BOTH branches — RouterLink when a target resolves, plain <span class=cmdk-option-link> otherwise — so an unresolvable entity renders a visible, named option instead of an empty zero-height <li>. Pinned by a new test (''still renders the option text when there is no resolvable href''), mutation-verified: removing the v-else fallback fails it. The optionLinks() helper was scoped to ''a.cmdk-option-link'' so it distinguishes the link from the span.'
status: addressed
---

**Finding (C1, critical).** In `CommandPaletteModal.vue:288-309` the `<li
role="option">` renders unconditionally from `v-for`, but `v-if` guards only the
inner `RouterLink` — and all three text spans live inside that link. When
`entityDetailHref` returns `''` (empty type or id) the result is a completely
empty option:

```html
<li id="cmdk-option-X" class="cmdk-option" role="option" aria-selected="false"></li>
```

Zero height, because the padding moved to `.cmdk-option-link`. This is WORSE
than the pre-change behaviour, where such a row at least rendered its text and
was merely inert.

Consequences: the visible row count disagrees with the result count;
`moveHighlight` still counts the invisible option so arrow-keying appears to
skip; `scrollHighlightedIntoView` scrolls to a zero-height element; a screen
reader announces an option with no accessible name.

**Resolution:** render the spans in both branches — `RouterLink` when a target
exists, plain `<span class="cmdk-option-link">` otherwise. This is the
conditional-anchor pattern already used correctly in IssuesTable, EntityList and
EntityDetail; the palette was the one place it was missed.
