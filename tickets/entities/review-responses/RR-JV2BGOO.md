---
id: RR-JV2BGOO
type: review-response
title: pgexec.rebind turns every ? into a placeholder
finding: A future query with a JSONB ? operator or a '?' literal would be renumbered.
severity: nit
resolution: sqlexec.Querier godoc now states a query holds no ? other than a placeholder.
status: addressed
---
