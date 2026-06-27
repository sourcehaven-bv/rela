---
id: RR-UD1B
type: review-response
title: Rrule helper already exists in utils/format.ts -- do not extract from RruleBuilder
finding: |
  utils/format.ts already does RRule.fromString(...).toText() for rrule values. RruleBuilder.vue has a near-duplicate inside a computed 'preview'. The ticket plans to "extract the helper from RruleBuilder" into widgets/rruleSummary.ts -- that would be the third copy. RruleBuilder's 'preview' is also tied to its internal form state, not a free function -- extracting it isn't a no-op.
severity: critical
resolution: |
  Plan revised. Drop the proposed widgets/rruleSummary.ts extraction. RruleWidget display mode calls formatValue(value, 'rrule') from frontend/src/utils/format.ts directly. The cards/list rrule rendering change (raw rrule -> human summary) is documented as deliberate behaviour delta #3 and #4. Cleaning up RruleBuilder.vue's duplicate preview helper is explicitly scoped out -- separate refactor.
status: addressed
---
