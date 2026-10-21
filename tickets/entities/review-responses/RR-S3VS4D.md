---
id: RR-S3VS4D
type: review-response
title: The `feature` fixture fix exposes a write/scan asymmetry that the cache used to hide
finding: 'internal/store/fsstore/persistence_test.go:31 adds `feature` to defaultTestSchemas because convert_test.go:27 creates FEAT-050 with type `feature`. That type was undeclared, and the test passed on develop only because a clean Close cached `{type: feature}` and the next open trusted it. The fixture change is correct, because Config.Schemas must be complete, so it masks nothing in this diff. But the root asymmetry is still there. layout.go:41-46 entityFileKey falls back to `type+"s"` for an undeclared type, so CreateEntity succeeds. layout.go:100-105 resolveEntityType returns "" for that directory, so the next open silently drops the entity. Before this change that happened only after a crash; now it happens on every reopen. entitymanager validates types upstream, so production is not affected today, but any direct store caller (migration, import, a test) gets a successful write that vanishes. Make the write path reject a type missing from l.schemas (CreateEntity, UpdateEntity, ReplaceEntityFamily) with a clear error, or file a follow-up ticket for it.'
severity: minor
reason: 'Out of scope for this change: the write fallback in entityFileKey predates it and changing write behavior needs its own design; filed as TKT-C2YVX1. The feature entry in the test schema stays, because convert_test.go writes that type.'
status: wont-fix
---
