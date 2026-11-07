---
id: RR-USR9S37
type: review-response
title: 'Code: post-migration drain cost undocumented'
finding: The drain after 0042 updates every row once outside HOT, which costs index entries and WAL on a large tenant.
severity: minor
resolution: '0042 now states the cost: one non-HOT update per row, a WAL burst and up to one dead row version per row, spread over sweep ticks.'
status: addressed
---
