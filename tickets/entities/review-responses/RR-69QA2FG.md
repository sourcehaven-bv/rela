---
id: RR-69QA2FG
type: review-response
title: Margin hard-codes the sticky top
finding: The 16px margin only happens to equal top 8px plus 8px of air; changing either would bring the overlap back.
severity: significant
resolution: A local --mobile-menu-btn-top sets top, and the margin is calc(top + --rl-space-2).
status: addressed
---
