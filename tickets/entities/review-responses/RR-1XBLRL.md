---
id: RR-1XBLRL
type: review-response
title: Rename count did one store read per neighbor
finding: Each distinct neighbor cost a Family read plus gate calls.
severity: minor
resolution: 'Endpoint headers now come from one ListEntityHeaders query. The gate is still asked per stored row: free under a global grant, one query per row under a relation-scoped grant. Full batching needs a ReadableFacesMany method on Declarative.'
status: addressed
---
