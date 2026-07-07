---
id: RR-KQ1DXS
type: review-response
title: Long labels overflow badges/options; Badge has no max-width/ellipsis
finding: Badge.vue has no max-width/ellipsis (lines 51-59) and `<option>` cannot wrap. A long label will blow out list columns and kanban cards. Add CSS max-width + ellipsis on .badge as part of this work, or explicitly accept the risk. Also document that `:value` on Badge stays the raw wire value (color key) and label is a separate concern, so nobody 'simplifies' by passing label as :value and breaking color for multi-word labels.
severity: minor
status: open
---
