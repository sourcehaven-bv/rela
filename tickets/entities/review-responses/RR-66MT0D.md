---
id: RR-66MT0D
type: review-response
title: 'StatusControl edge case: warning is unbuildable, and the interaction is two-axis'
finding: 'PLAN-DMQFRJ Edge Cases said `widget:` on a state-machine field is inert because ''StatusControl wins'', and specced a config-load warning. Two problems. (1) The interaction is two-axis, not inert: the guard at SectionEditForm.vue:279 is `row.field.transitions !== undefined && row.field.render === ''input''`, so StatusControl wins ONLY on render:input. On render:display a machine field deliberately falls through to the display arm (comment at :273-277, TKT-HOIX1) and DOES use row.widget. So `widget:` is inert on input, live on display -- the plan described only half. (2) The warning is not implementable. Machine-ness is not declared in the metamodel (no state-machine types in internal/metamodel/types.go); it is a runtime per-entity, per-principal value produced by computeTransitions (affordances.go:1051) via a TransitionResolver type-assertion (affordances.go:179) that Nop/Demo resolvers don''t implement. CollectConfigWarnings(cfg, meta) has neither a resolver nor an entity, so it cannot know a field is a machine field.'
severity: significant
resolution: 'Verified: no state-machine declaration in metamodel/types.go; machine-ness originates from the runtime TransitionResolver assertion. Dropped the unbuildable warning from the acceptance criteria rather than shipping an AC that cannot be implemented. Instead documenting the two-axis interaction in docs/data-entry.md (widget inert on render:input where StatusControl owns the field, honoured on render:display) and adding a component test pinning both halves.'
status: addressed
---
