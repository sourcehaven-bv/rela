---
id: RR-IVL3SL
type: review-response
title: Overlong doc comment line in InboxView
finding: frontend/src/views/InboxView.vue:9 runs to about 130 characters, because the new sentence was joined to the old one. Reflow the comment block.
severity: nit
resolution: ADDRESSED reflowed the InboxView doc comment into two paragraphs.
status: addressed
---
