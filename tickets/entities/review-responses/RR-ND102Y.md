---
id: RR-ND102Y
type: review-response
title: Two different RelationKey names (sync manifest func and entity type)
finding: internal/cli/sync/state.go has func RelationKey(from, relType, to) string with a different format, now sharing a name with entity.RelationKey.
severity: nit
reason: PR 7 moves relation methods onto entity.RelationKey and touches sync; rename the manifest helper there.
status: deferred
---
