---
id: RR-NLALX2S
type: review-response
title: Visible notice row folds away on the next append
finding: A notice arrives last in its append, renders as a row and joins the fold on the next quiet append, on every move. Focus on that row is lost and the scroll anchor falls back to the fold button; the plan had no criterion for it.
severity: significant
resolution: 'Plan: RlChannel hands focus to the fold button that now holds a focused row that folded away; the anchor uses the fold line. AC 7a with integration tests for focus and scroll offset.'
status: addressed
---
