---
id: RR-BXLGOA
type: review-response
title: describedBy duplicates the template's render conditions
finding: RlChannelMessage.vue:130-132 recomputes whether the card head (headId) is rendered with `!prose && messageType`. The template's own branches (v-if prose / v-else-if messageType || shown.length || message.action) decide that. If they drift, aria-describedby points at a missing id. Keep this in mind; a comment tying the two together would help.
severity: nit
resolution: Added a comment on decisionsDescribedBy that ties its condition to the template branch that renders headId.
status: addressed
---
