---
id: RR-CBS2QW
type: review-response
title: 'Focus ring hardcoded indigo (#6366f1) — renders off-hue and does not follow the theme'
finding: 'The focus ring was `box-shadow: 0 0 0 2px rgb(99 102 241 / 30%)` with no `var()` at all. This is distinct from the sibling widgets'' `var(--accent-color, #6366f1)` pattern, where the indigo is a FALLBACK that never renders (tokens.css defines --accent-color unconditionally for both themes). Here the literal IS the value: it renders on every focus, in both themes, against an app whose real accent is #4772fb (light) / #6f93ff (dark). It also defeats operator theming — the `@layer rela` architecture exists so an unlayered `custom.css` accent wins, but a literal is unreachable by it. So "consistency with siblings" does not defend this line; it propagates the one part of the copied prior art that was a mistake.'
severity: significant
resolution: 'Replaced with `color-mix(in srgb, var(--accent-color) 30%, transparent)`. Verified in-browser that it resolves to `color(srgb 0.435294 0.576471 1 / 0.3)` = #6f93ff at 30% — the live dark-theme accent, not indigo — and confirmed the function survives the Vite build unmangled.'
status: addressed
---

The `var(--accent-color, #6366f1)` fallbacks elsewhere in the rule set were
deliberately left alone: they are dead-but-harmless and match every sibling
widget. Changing them is a separate cleanup, not this ticket.

`RelationCards.vue:998` carries the same hardcoded ring. Not fixed here —
see [[RR-CBLEV8]] for why that is deferred rather than done in passing.
