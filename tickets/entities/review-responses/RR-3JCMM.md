---
id: RR-3JCMM
type: review-response
title: Add :focus-visible style for keyboard a11y, not just verify in test
finding: 'Plan adds text-decoration: none + color: inherit to .list-link, killing the visual link affordance. For keyboard users, add an explicit :focus-visible outline rule in CSS, not just a manual verification step. Manual a11y check verifies; CSS provides.'
severity: nit
resolution: 'Added :focus-visible CSS rule (outline: 2px solid var(--accent-color); outline-offset: 2px;) to .list-link in plan. Manual a11y check verifies; CSS provides the visible affordance.'
status: addressed
---
