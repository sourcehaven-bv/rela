---
id: RR-CBM4WP
type: review-response
title: 'Dropped -webkit-appearance relative to the copied prior art, with no autoprefixer to regenerate it'
finding: 'RelationCards writes both `appearance: none` and `-webkit-appearance: none`; this change kept only the unprefixed form. There is no postcss.config, no browserslist key, and no autoprefixer dependency, so nothing regenerates the prefix. Functionally fine — unprefixed `appearance` is supported from Safari 15.4 (2022), inside Vite''s default ES-modules baseline — but it reads as an oversight rather than a decision.'
severity: nit
resolution: 'Kept unprefixed, with a comment recording the reasoning and noting that RelationCards predates the baseline. Verified independently: no browserslist config and no autoprefixer in the build.'
status: addressed
---

Documenting the decision was the point of the finding; the code is unchanged
apart from the comment.
