---
id: RR-1H5O52N
type: review-response
title: Mentioned-by hide flag wired to the wrong panel
finding: EntityDetail.vue passed hide-id to DocumentsPanel instead of MentionedByPanel, so mentioned_by_hide_id had no effect.
severity: critical
resolution: Moved the prop to MentionedByPanel and added a test that asserts the panel receives it.
status: addressed
---
