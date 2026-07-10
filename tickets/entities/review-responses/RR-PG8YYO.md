---
id: RR-PG8YYO
type: review-response
title: Stale 'upserts' godoc on autocascade.Host.WriteRelation
finding: autocascade/host.go WriteRelation godoc still said 'upserts the relation to the store' after the cascadeHost impl became create-or-noop, misleading a future implementer into thinking upsert is part of the contract.
severity: nit
resolution: Updated the godoc to 'create-then-noop-on-conflict, never an update — cascade relations are property-less'. No behavior change. Commit c95947da.
status: addressed
---
