---
id: RR-S2H5R3
type: review-response
title: Resolver opened twice in MapWebhook
finding: mapFor and the check each call ForPrincipal.
severity: nit
reason: One extra Request per CLI run; the cost is a single Globals walk.
status: wont-fix
---
