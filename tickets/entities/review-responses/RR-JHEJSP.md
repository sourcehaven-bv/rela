---
id: RR-JHEJSP
type: review-response
title: Store wired into alias fanout
finding: 'Architecture design review: rename/delete should also wake subscribers.'
severity: significant
resolution: 'Plan: Service implements EntityRenamed/EntityDeleted like piles.Service, passed to newAliasFanout.'
status: addressed
---
