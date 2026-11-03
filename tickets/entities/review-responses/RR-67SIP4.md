---
id: RR-67SIP4
type: review-response
title: grantRows evaluates every conditional grant eagerly
finding: grantRows runs one MatchingFaces per conditional grant per type even when every hit on the page is already admitted.
severity: nit
reason: It runs once per page, so it costs a constant number of queries. Skipping it would make the query count depend on the data, which the budget test would then have to model.
status: wont-fix
---
