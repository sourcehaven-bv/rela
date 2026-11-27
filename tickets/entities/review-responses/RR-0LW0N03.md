---
id: RR-0LW0N03
type: review-response
title: Hidden ID also dropped the face suffix
finding: With hide ID on, a mention of a faced entity lost its @face, so two faces of one entity looked the same.
severity: significant
resolution: shownAddress keeps @face when it hides the ID; pinned in MentionedByPanel.test.ts.
status: addressed
---
