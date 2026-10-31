---
id: RR-L2BF2WL
type: review-response
title: Quick successive drops on one loose card create two groups
finding: groupOnto had no in-flight guard; a second drop on the same loose card before the first finished created a second group and split the cards.
severity: significant
resolution: 'Drops are chained on a promise and each re-reads the cards by id after the previous one refreshed. Test: ''handles quick drops on one loose card one at a time, making one group''.'
status: addressed
---
