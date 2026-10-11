---
id: RR-T2GYZF
type: review-response
title: 'where: filters after the cap'
finding: Filtering the capped window can hide matching sources beyond the first 200 ids and show an empty section.
severity: significant
resolution: where filters page by page while collecting, so the 200 are matching sources. TestViewMentions_CapAndWhere covers matches past the first page.
status: addressed
---
