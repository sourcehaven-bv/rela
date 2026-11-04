---
id: RR-C9R3KO
type: review-response
title: Impression watcher misses a replacement suggestion
finding: NextActionCard and SidebarFooter watch a boolean; X->Y at the same prominence never calls markShown for Y.
severity: significant
resolution: 'Planned: watch the suggestion key together with the prominence flag in both components; unit test for an X->Y swap.'
status: addressed
---
