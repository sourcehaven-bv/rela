---
id: RR-6Z0XJH
type: review-response
title: Rejected-reload restore leaves ghost index entries of the rolled-back type
finding: 'internal/cli/mcp_wiring.go:187. RefreshStoreSchema(next) has already scanned the new type''s directory into s.entities. restoreStoreSchema(old) only narrows declared, and paths keeps the type, so the old services in force now list and get entities of a type their metamodel does not declare. Before the reload those files were invisible. Concurrent writes are otherwise safe: old''s entitymanager rejects the new type, and the store refuses it after restore. A write of a type that next removes fails spuriously in the window between refresh and publish, but that is transient. Fix: refresh after publish succeeds (publish is cheap and under mu; on refresh failure, republish old), or have SetSchemas return an undo that also drops the entries it added. Extend TestMCPServices_RejectedReloadRestoresStoreSchema to seed a file in entities/riskregister and assert GetEntity(RSK-9) is not found after the rejection.'
severity: minor
resolution: The store is refreshed only after publish succeeds; the restore step is removed.
status: addressed
---
