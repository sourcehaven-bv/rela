---
id: RR-EBA0L
type: review-response
title: Delete prompt lost <strong>{{ id }}</strong> emphasis silently
finding: Old EntityDetail/EntityList delete prompts emphasized the entity id with <strong>. New plain-string message renders the id inline. This is a UX regression that wasn't flagged anywhere. Either acknowledge as an accepted minor regression with a follow-up ticket, or address (e.g., wrap id in backticks for monospace via CSS).
severity: significant
resolution: Wrapped entity id in single quotes in delete prompts (EntityList.vue and EntityDetail.vue) for visible emphasis without resorting to v-html / messageHtml. Less heavy than <strong> but unmistakable as a literal id.
status: addressed
---
