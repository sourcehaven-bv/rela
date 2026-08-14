---
id: RR-CR-LAYERTEST
type: review-response
title: 'TestBuiltCSSIsLayered used a flat offset comparison: too weak and too strong at once'
finding: "The test computed `strings.Index(css, '@layer rela {')` and flagged any `:root` at a greater offset. Since wrapCss always emits carved-out tokens before the layer and everything else after, the check only ever confirmed 'the carve-out happened' on the happy path. Worse, it would FAIL on a legitimately-layered `:root` nested inside `@media`/`@supports` — valid CSS that correctly belongs in the layer. So it could miss real corruption while failing on correct output."
severity: significant
status: addressed
resolution: "Replaced with `topLevelRootInsideLayer`, a depth-aware scanner that only flags a `:root` DECLARATION rule at the layer's own nesting depth, and ignores descendant selectors (`:root .fa-rotate-90` — Font Awesome ships this) and selector lists (`:root, .x`). Added TestTopLevelRootInsideLayer with 8 cases covering both directions, because a guard that cries wolf gets weakened by the next person who hits it."
---

Raised by `/code-review` of the TKT-3DBK6I implementation.
