---
id: RR-DILMO4
type: review-response
title: Three entity loads per field-visibility decision — concurrent-write snapshot race
finding: The generic path loaded the hit entity 3x (hidden-fields callback, Service.MatchedFields, body load), each a fresh atomic.Pointer snapshot. A concurrent edit to the matched property between loads could make the hidden-set load observe a state where a when:-predicate no longer marks it hidden, defeating fail-closed. Violates CLAUDE.md 'capture state once per operation.'
severity: significant
resolution: The seam now loads the entity ONCE in fieldVisible and threads that single snapshot into both the HiddenFieldsFunc (signature gained *entity.Entity) and prov.MatchedFields (now takes the entity, not an id). pgstore already reused its scanned row; the generic path matches it. hiddenSearchFields no longer loads.
status: addressed
---
