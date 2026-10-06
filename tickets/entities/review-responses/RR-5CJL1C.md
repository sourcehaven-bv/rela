---
id: RR-5CJL1C
type: review-response
title: Raw store writes bypass the owning rules
finding: Not every relation write goes through Manager.CreateRelation. Automation create_relation writes through the raw store (entitymanager/cascadehost.go:162). The data-entry soft-condition fallback writes raw after the manager refuses (dataentry/relations_modern.go:525). The importer and data migrations write raw by design, and on the fs backend files can be edited by hand or merged by git. Write-time checks alone cannot guarantee the invariants.
severity: significant
resolution: 'Plan: one checkOwningEdge called from Manager.CreateRelation, cascadeHost.WriteRelation and the data-entry soft-condition fallback, inside Store.Tx. Importer, migrations and hand edits stay operator-trusted; the read side tolerates their output (RR-65RROE).'
status: addressed
---
