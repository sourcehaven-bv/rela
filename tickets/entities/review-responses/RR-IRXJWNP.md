---
id: RR-IRXJWNP
type: review-response
title: An anchored card read the whole list and filtered in Go
finding: 'dashboard_card.go loaded every gated row of the card''s list and then kept the reached ids in Go. A table card lost its bounded page read. This breaks collection-reads.md: N cards on a large type cost N full scans per view.'
severity: significant
resolution: The reach now rides on gatedListQuery.reach and narrows the read by id in scopedSortedEntitiesScoped, as a page scope does. TestQueryBudget_EntityDashboardCardIsSizeIndependent pins 6 reads for count and table cards and checks the rows read do not grow with the store.
status: addressed
---
