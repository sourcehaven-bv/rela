---
id: RR-FRC6DR
type: review-response
title: 'A drag-over drop-target highlight was converted into a focus ring because it matched the same literal'
finding: '`.relation-card.card-drag-over` in RelationCards.vue is a pointer drag-hover state, not a focus indicator. It was authored at 25% alpha — deliberately heavier than the 0.1 focus rings — and the sweep converted it to the opaque `--focus-ring` plus a `--focus-ring-gap` band because it matched the same `rgba(99, 102, 241, …)` literal. Result: a drop target rendered visually louder than before AND semantically announced itself as "focused".'
severity: significant
resolution: 'Reverted to its own translucent weight, expressed as `color-mix(in srgb, var(--accent-color) 25%, transparent)` so it still follows the theme and an operator accent — the theming half of the ticket — without adopting focus semantics it does not have. Commented at the rule so the next sweep does not re-convert it.'
status: addressed
---

This is the concrete instance of a scope principle worth naming: **"the regex
matched" is not a scope criterion.** The six error rings were correctly pulled
in ([[RR-FRC2GD]]) because they share a root cause, a fix and a file set. The
four background tints and this drag state came along only because they shared a
*literal*. Three of the four tints were handled correctly by keeping them
translucent; this one was not, and it is the one where the mistake was visible
rather than cosmetic.
