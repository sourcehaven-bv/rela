---
id: RR-W3J1A
type: review-response
title: '''No behaviour change'' is unverifiable as written'
finding: 'Forms have e2e tests but they don''t cover keystroke-level behaviour: focus order, IME composition, autocomplete, label-click activation, number arrow keys, date keyboard nav. Refactor regressing any of these is a P1 the day after merge. Need explicit verification gate.'
severity: significant
resolution: 'Plan revised: explicit verification gate added — per-(propertyType, widget) snapshot test pre/post, e2e inventory + gap-fill before refactor, manual smoke on every metamodel entity type, original template preserved as commented code for one release. One-week bake before tickets 2-5 start. See TKT-MZSIJ ''Verification gate''.'
status: addressed
---
