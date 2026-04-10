---
id: RR-5LY67
type: review-response
title: Entity-type rename orchestration belongs on Workspace, not CLI
finding: 'cli/rename.go:168-194 applyRenameEntity does: update metamodel YAML, rename directory, call markdown.UpdateEntityTypesInDir, rename templates. The plan wraps just the file-rewrite step but leaves CLI doing low-level filesystem orchestration. The whole entity-type rename should be a Workspace method, same pattern as entity-ID rename (workspace.Rename).'
severity: critical
resolution: 'Plan updated: will add Workspace.RenameEntityType(old, new, plural string, opts) that owns the entire multi-step entity-type rename operation (metamodel update, directory rename, file rewrite, template rename, cache invalidation). CLI becomes a thin caller. This eliminates the UpdateEntityTypesInDir wrapper entirely.'
status: addressed
---
