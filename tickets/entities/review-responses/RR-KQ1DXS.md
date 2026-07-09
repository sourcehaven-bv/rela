---
id: RR-KQ1DXS
type: review-response
title: Long labels overflow badges/options; Badge has no max-width/ellipsis
finding: Badge.vue has no max-width/ellipsis (lines 51-59) and `<option>` cannot wrap. A long label will blow out list columns and kanban cards. Add CSS max-width + ellipsis on .badge as part of this work, or explicitly accept the risk. Also document that `:value` on Badge stays the raw wire value (color key) and label is a separate concern, so nobody 'simplifies' by passing label as :value and breaking color for multi-word labels.
severity: minor
resolution: 'Badge .badge gained max-width: 24ch + overflow ellipsis + nowrap. The :value=raw-value / label-is-separate guardrail is documented in Badge.vue comments and the metamodel docs; store getter keeps color keyed on value. Verified by Badge.test.ts ''keeps color styling keyed on the raw value''.'
status: addressed
---
