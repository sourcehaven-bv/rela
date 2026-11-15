---
id: RR-J7LWBZF
type: review-response
title: Git sync enabled logged for tenants
finding: Startup logged git sync enabled for each tenant before tenant mode turned git sync off, which misled operators.
severity: nit
resolution: 'Fixed in 13cd61e18: the log moved into NewRouter so it reflects the final state.'
status: addressed
---
