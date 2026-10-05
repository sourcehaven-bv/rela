---
id: RR-ND102Y
type: review-response
title: Two different RelationKey names (sync manifest func and entity type)
finding: internal/cli/sync/state.go has func RelationKey(from, relType, to) string with a different format, now sharing a name with entity.RelationKey.
severity: nit
reason: PR 7 left the sync helper as is. The two names live in different packages (sync.RelationKey returns a manifest string and entity.RelationKey is a type) and every call site is package-qualified so a reader cannot confuse them.
status: wont-fix
---
