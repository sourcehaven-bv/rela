---
id: RR-H1TUEQ
type: review-response
title: default_sort is applied in three client places
finding: listBaseParams, entityTarget and useScopeNavigation each add default_sort; navigator would walk another order.
severity: significant
resolution: 'Plan updated: One helper effectiveDefaultSort used by all three; AC that navigator order matches tab order.'
status: addressed
---
