---
id: RR-VBJ91V
type: review-response
title: '`display: content` sections build wire fields the SPA never renders; classification and builder disagree'
finding: '`sectionDisplayModesRenderingFields` (validate.go) lists properties/list/cards, excluding `content`. But `buildSections` runs `case "content", "cards":` through `buildSectionEntityData(..., sec.Render)` (sections.go:329-336), so a traverse-sourced content section DOES build SectionFieldData with a resolved Render and ships it on the wire. Three in-repo content sections carry fields this way. Net user-visible effect is correct because the SPA''s content-card template renders only the markdown body, but the classification and the builder disagree and a future reader will trip on it.'
severity: minor
resolution: 'Kept the classification (it correctly tracks what is RENDERED, which is what an operator setting `render:` cares about) and documented the discrepancy at the map declaration: content shares a builder arm with cards, the SPA ignores those fields, and adding `content` to the map would suppress a warning the operator needs. Explicitly warns against ''fixing'' the mismatch that way.'
status: addressed
---

Verified the SPA side before deciding: `EntityDetail.vue:932`
(`v-else-if="section.display === 'content' && section.entities?.length"`)
renders `content-card` elements whose template emits only the markdown body — no
`card-fields`, no field loop. So the built fields are genuinely inert payload on
that surface.

Two candidate fixes were considered:
1. Add `content` to the map — rejected: it would silence a warning for a mode where `render:`
really does nothing, which is the opposite of the warning's purpose.
2. Stop building fields for `content` — rejected as out of scope: it would change the wire
shape for a pre-existing behaviour unrelated to this ticket, with unknown
consumers.

Documenting the seam is the honest option. Worth a follow-up ticket if the
wasted payload matters.
